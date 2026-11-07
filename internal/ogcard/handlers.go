package ogcard

import (
	"context"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"lumo-api-gateway/internal/downstream"
)

// record is the part of a movie's / series' details the card uses.
type record struct {
	Title         string  `json:"title"`
	OriginalTitle *string `json:"originalTitle"`
	ReleaseDate   *string `json:"releaseDate"`
	TmdbRating    any     `json:"tmdbRating"`
	Genres        *string `json:"genres"`
	PosterImgURL  *string `json:"posterImgURL"`
	BackdropURL   *string `json:"backgroundImg_URL"`
	HasBgAudio    bool    `json:"hasBgAudio"`
}

// genres in Bulgarian, as my-app's GENRE_TRANSLATIONS.
var genresBG = map[string]string{
	"Action": "Екшън", "Horror": "Ужаси", "Comedy": "Комедия", "Drama": "Драма", "Mystery": "Мистерия",
	"Romance": "Романтика", "Adventure": "Приключенски", "Crime": "Криминален", "Fantasy": "Фентъзи",
	"Historical": "Исторически", "Animation": "Анимация", "Thriller": "Трилър", "Western": "Уестърн",
	"Biography": "Биографичен", "Documentary": "Документален", "Family": "Семеен", "Musical": "Мюзикъл",
	"Sports": "Спортен", "War": "Военен", "Science Fiction": "Научна фантастика", "Music": "Музикален",
	"History": "Исторически", "TV Movie": "Телевизионен филм", "Action & Adventure": "Екшън и приключения",
	"Sci-Fi & Fantasy": "Фантастика и фентъзи", "War & Politics": "Война и политика", "Kids": "Детски",
	"Reality": "Риалити", "Soap": "Сапунена опера", "Talk": "Ток шоу", "News": "Новини",
}

const tmdbImageBase = "https://image.tmdb.org/t/p"

// cacheSize cards (~60-120 KB each) stay in memory: a link shared in a group
// is fetched by every member's app within minutes.
const cacheSize = 400

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type entry struct {
	jpeg []byte
	at   time.Time
}

type Handlers struct {
	ds        *downstream.Client
	moviesURL string
	seriesURL string
	images    *http.Client

	mu    sync.Mutex
	cache map[string]entry
}

func NewHandlers(ds *downstream.Client, moviesSvcURL, seriesSvcURL string) *Handlers {
	return &Handlers{
		ds:        ds,
		moviesURL: moviesSvcURL,
		seriesURL: seriesSvcURL,
		images:    &http.Client{Timeout: 8 * time.Second},
		cache:     map[string]entry{},
	}
}

// Register: public, like the pages the cards belong to.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /og/movie/{id}", func(w http.ResponseWriter, r *http.Request) { h.serve(w, r, "movie") })
	mux.HandleFunc("GET /og/series/{id}", func(w http.ResponseWriter, r *http.Request) { h.serve(w, r, "series") })
}

const cacheTTL = 24 * time.Hour

func (h *Handlers) serve(w http.ResponseWriter, r *http.Request, kind string) {
	id := strings.TrimSuffix(r.PathValue("id"), ".jpg")
	if !idPattern.MatchString(id) {
		http.NotFound(w, r)
		return
	}
	key := kind + ":" + id

	h.mu.Lock()
	cached, ok := h.cache[key]
	h.mu.Unlock()
	if !ok || time.Since(cached.at) > cacheTTL {
		card, err := h.card(r.Context(), kind, id)
		if err != nil {
			log.Printf("og card %s: %v", key, err)
			http.NotFound(w, r)
			return
		}
		jpeg, err := Render(card)
		if err != nil {
			log.Printf("og card %s: %v", key, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		cached = entry{jpeg: jpeg, at: time.Now()}
		h.remember(key, cached)
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(cached.jpeg)))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(cached.jpeg)
}

func (h *Handlers) remember(key string, e entry) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.cache) >= cacheSize {
		var oldestKey string
		var oldest time.Time
		for k, v := range h.cache {
			if oldestKey == "" || v.at.Before(oldest) {
				oldestKey, oldest = k, v.at
			}
		}
		delete(h.cache, oldestKey)
	}
	h.cache[key] = e
}

var errNotFound = &httpError{"not found"}

type httpError struct{ msg string }

func (e *httpError) Error() string { return e.msg }

func (h *Handlers) card(ctx context.Context, kind, id string) (Card, error) {
	var u string
	if kind == "movie" {
		u = h.moviesURL + "/get-movie-details?" + url.Values{"movieId": {id}, "lang": {"bg"}}.Encode()
	} else {
		u = h.seriesURL + "/get-series-details?" + url.Values{"id": {id}, "lang": {"bg"}}.Encode()
	}
	res, err := h.ds.Do(ctx, http.MethodGet, u, nil, nil)
	if err != nil {
		return Card{}, err
	}
	if !res.OK() {
		return Card{}, errNotFound
	}
	var rec record
	if err := json.Unmarshal(res.Body, &rec); err != nil || strings.TrimSpace(rec.Title) == "" {
		return Card{}, errNotFound
	}
	return h.toCard(ctx, rec, kind), nil
}

func (h *Handlers) toCard(ctx context.Context, rec record, kind string) Card {
	card := Card{Title: strings.TrimSpace(rec.Title), Badges: []string{"БГ субтитри"}}
	if rec.HasBgAudio {
		card.Badges = append(card.Badges, "БГ аудио")
	}

	var sub []string
	if o := deref(rec.OriginalTitle); o != "" && o != card.Title {
		sub = append(sub, o)
	}
	if year := deref(rec.ReleaseDate); len(year) >= 4 && isDigits(year[:4]) {
		sub = append(sub, year[:4])
	}
	if kind == "movie" {
		sub = append(sub, "Филм")
	} else {
		sub = append(sub, "Сериал")
	}
	card.Subtitle = strings.Join(sub, " · ")

	switch v := rec.TmdbRating.(type) {
	case string:
		card.Rating, _ = strconv.ParseFloat(strings.TrimSpace(v), 64)
	case float64:
		card.Rating = v
	}

	for _, g := range strings.Split(deref(rec.Genres), ",") {
		g = strings.Trim(strings.TrimSpace(g), `"'.`)
		if g == "" {
			continue
		}
		if bg, ok := genresBG[g]; ok {
			g = bg
		}
		card.Genres = append(card.Genres, g)
		if len(card.Genres) == 3 {
			break
		}
	}

	poster := deref(rec.PosterImgURL)
	backdrop := deref(rec.BackdropURL)
	if backdrop == "" {
		backdrop = poster
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); card.Poster = h.fetchImage(ctx, poster, "w342") }()
	// Small on purpose: it's blurred anyway.
	go func() { defer wg.Done(); card.Backdrop = h.fetchImage(ctx, backdrop, "w300") }()
	wg.Wait()
	return card
}

// fetchImage gets a TMDB image (a path or a full URL), or nil.
func (h *Handlers) fetchImage(ctx context.Context, path, size string) image.Image {
	if path == "" {
		return nil
	}
	u := path
	if !strings.HasPrefix(path, "http") {
		u = tmdbImageBase + "/" + size + path
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil
	}
	resp, err := h.images.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	img, _, err := image.Decode(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil
	}
	return img
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}
