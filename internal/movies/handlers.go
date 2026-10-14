package movies

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"lumo-api-gateway/internal/clerkauth"
	"lumo-api-gateway/internal/downstream"
	"lumo-api-gateway/internal/models"
	"lumo-api-gateway/internal/users"
)

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// Handlers mirrors MovieController + MovieCommentsController +
// TrendingMoviesController — same paths, same params, same status codes.
type Handlers struct {
	client     *Client
	userClient *users.Client
}

func NewHandlers(client *Client, userClient *users.Client) *Handlers {
	return &Handlers{client: client, userClient: userClient}
}

// Register wires every route onto mux — mirrors the @GetMapping/@PostMapping/
// etc. annotations exactly, path for path.
func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /get-trending-movies", h.getTrendingMovies)

	mux.HandleFunc("POST /add-movies-by-actor/{imdb_id}", h.addMoviesByActor)
	mux.HandleFunc("POST /search-movies", h.searchMovies)
	mux.HandleFunc("GET /search-movies-matching-results/{searchTitle}", h.searchMoviesMatchingResults)
	mux.HandleFunc("GET /get-searched-movies-count", h.getSearchedMoviesCount)
	mux.HandleFunc("GET /get-movies-count-by-genre", h.findMoviesCountByGenre)
	mux.HandleFunc("GET /get-next-twenty-movies-by-genre", h.getNextTwentyMoviesByGenre)
	mux.HandleFunc("GET /get-next-thirty-movies", h.getEveryThirtyMovies)
	mux.HandleFunc("GET /get-movies-by-title", h.getMoviesByTitle)
	mux.HandleFunc("GET /get-movie-details", h.getConcreteMovieDetails)
	mux.HandleFunc("GET /get-movie-titles", h.getMovieTitles)
	mux.HandleFunc("GET /get-all-movies-count", h.getAllMoviesCount)
	mux.HandleFunc("GET /discover-movies", h.discoverMovies)
	mux.HandleFunc("GET /discover-movies-count", h.discoverMoviesCount)
	mux.HandleFunc("GET /suggest-movie-actors", h.suggestMovieActors)

	mux.HandleFunc("DELETE /delete-movie-comment", h.deleteMovieComment)
	mux.HandleFunc("PUT /update-movie-comment", h.updateMovieComment)
	mux.HandleFunc("GET /get-next-10-movie-comments", h.getNext10Comments)
	mux.HandleFunc("POST /post-movie-comment", h.postComment)
	mux.HandleFunc("PUT /react-movie-comment", h.reactToMovieComment)
	mux.HandleFunc("GET /get-movie-comments-reactions", h.getMovieCommentsReactions)
}

// proxyOr500 is the pure-passthrough shape most GET endpoints share: no
// try/catch in the Java controller, so any downstream failure falls through
// to Spring's default unhandled-exception behavior (500), and success is
// forwarded byte-for-byte.
func proxyOr500(w http.ResponseWriter, label string, res *downstream.Result, err error) {
	if err != nil {
		log.Printf("%s: %v", label, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !res.OK() {
		log.Printf("%s: downstream status %d: %s", label, res.StatusCode, res.Body)
		downstream.WriteProxy(w, res)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) getTrendingMovies(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.GetTrendingMovies(r.Context())
	proxyOr500(w, "GET /get-trending-movies", res, err)
}

func (h *Handlers) addMoviesByActor(w http.ResponseWriter, r *http.Request) {
	imdbID := r.PathValue("imdb_id")
	res, err := h.client.AddMoviesByActor(r.Context(), imdbID)
	if err != nil || !res.OK() {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) searchMovies(w http.ResponseWriter, r *http.Request) {
	var req models.CinemaRecRequestDto
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.client.SearchMovies(r.Context(), req.RecordName)
	if err != nil {
		log.Printf("POST /search-movies: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !res.OK() {
		log.Printf("POST /search-movies: downstream status %d", res.StatusCode)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) searchMoviesMatchingResults(w http.ResponseWriter, r *http.Request) {
	searchTitle := r.PathValue("searchTitle")
	limit := queryInt(r, "limit", 10)
	offset := queryInt(r, "offset", 0)
	res, err := h.client.SearchMoviesMatchingResults(r.Context(), searchTitle, limit, offset)
	proxyOr500(w, "GET /search-movies-matching-results", res, err)
}

func (h *Handlers) getSearchedMoviesCount(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.GetSearchedMoviesCount(r.Context(), r.URL.Query().Get("title"))
	proxyOr500(w, "GET /get-searched-movies-count", res, err)
}

func (h *Handlers) findMoviesCountByGenre(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.FindMoviesCountByGenre(r.Context(), r.URL.Query().Get("receivedGenre"))
	proxyOr500(w, "GET /get-movies-count-by-genre", res, err)
}

func (h *Handlers) getNextTwentyMoviesByGenre(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := queryOr(q, "page", "0")
	size := queryOr(q, "size", "20")
	res, err := h.client.GetNextTwentyMoviesByGenre(r.Context(), page, size, q.Get("receivedGenre"))
	proxyOr500(w, "GET /get-next-twenty-movies-by-genre", res, err)
}

func (h *Handlers) getEveryThirtyMovies(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := queryOr(q, "page", "0")
	size := queryOr(q, "size", "10")
	res, err := h.client.GetEveryThirtyMovies(r.Context(), page, size)
	proxyOr500(w, "GET /get-next-thirty-movies", res, err)
}

// getMoviesByTitle mirrors MovieController.getMoviesByTitle's blank-title
// short-circuit: an empty/whitespace title returns an empty array without
// ever calling movies-svc.
func (h *Handlers) getMoviesByTitle(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")
	if isBlank(title) {
		writeJSON(w, []any{})
		return
	}
	res, err := h.client.GetMoviesByTitle(r.Context(), title)
	proxyOr500(w, "GET /get-movies-by-title", res, err)
}

// getConcreteMovieDetails mirrors MovieController.getConcreteMovieDetails's
// blank-movieId short-circuit: returns a zero-value Movie object without
// ever calling movies-svc.
func (h *Handlers) getConcreteMovieDetails(w http.ResponseWriter, r *http.Request) {
	movieID := r.URL.Query().Get("movieId")
	if isBlank(movieID) {
		writeJSON(w, emptyMovie)
		return
	}
	res, err := h.client.GetConcreteMovieDetails(r.Context(), movieID)
	proxyOr500(w, "GET /get-movie-details", res, err)
}

func (h *Handlers) getAllMoviesCount(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.GetAllMoviesCount(r.Context())
	proxyOr500(w, "GET /get-all-movies-count", res, err)
}

func (h *Handlers) discoverMovies(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := queryOr(q, "page", "0")
	size := queryOr(q, "size", "30")
	res, err := h.client.DiscoverMovies(r.Context(), q.Get("genre"), q.Get("year"), q.Get("actorName"), q.Get("sort"), page, size)
	proxyOr500(w, "GET /discover-movies", res, err)
}

func (h *Handlers) discoverMoviesCount(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := h.client.DiscoverMoviesCount(r.Context(), q.Get("genre"), q.Get("year"), q.Get("actorName"))
	proxyOr500(w, "GET /discover-movies-count", res, err)
}

func (h *Handlers) suggestMovieActors(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.SuggestMovieActors(r.Context(), r.URL.Query().Get("query"))
	proxyOr500(w, "GET /suggest-movie-actors", res, err)
}

// --- Comments endpoints (protected: delete/update/post/react) ---

func (h *Handlers) deleteMovieComment(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	requesterID := clerkauth.ClerkUserID(r)

	res, err := h.client.DeleteMovieComment(r.Context(), q.Get("commentId"), q.Get("movieId"), requesterID)
	writeCommentMutationResult(w, "DELETE /delete-movie-comment", res, err)
}

func (h *Handlers) updateMovieComment(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	requesterID := clerkauth.ClerkUserID(r)

	var req models.UpdateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	res, err := h.client.UpdateMovieComment(r.Context(), q.Get("commentId"), q.Get("movieId"), req.CommentText, requesterID)
	writeCommentMutationResult(w, "PUT /update-movie-comment", res, err)
}

func (h *Handlers) getNext10Comments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := h.client.GetNext10Comments(r.Context(), q.Get("order"), q.Get("currentCinemaRecordId"))
	proxyOr500(w, "GET /get-next-10-movie-comments", res, err)
}

// postComment mirrors MovieCommentsController.postComment: display
// name/avatar come from lumo-user-svc's own record (never from client
// input), and the author id is the already-verified Clerk session — not
// anything the client body claims to be.
func (h *Handlers) postComment(w http.ResponseWriter, r *http.Request) {
	clerkID := clerkauth.ClerkUserID(r)

	var req models.PostCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userRes, err := h.userClient.GetCurrentUser(r.Context(), clerkID)
	if err != nil {
		log.Printf("Failed to post movie comment for clerkId=%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if userRes.StatusCode == http.StatusNotFound {
		// lumo-user-svc doesn't have this clerkId yet — the Clerk webhook
		// that syncs new accounts can race a brand new signup.
		w.WriteHeader(http.StatusConflict)
		return
	}
	if !userRes.OK() {
		log.Printf("Failed to post movie comment for clerkId=%s: user-svc status %d", clerkID, userRes.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	user, err := users.ParseUserResponse(userRes)
	if err != nil {
		log.Printf("Failed to post movie comment for clerkId=%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := h.client.PostComment(r.Context(), user.DisplayName, user.DisplayName, user.ProfileImageURL, req.CommentText, createdAt, req.CinemaRecordID, clerkID)
	if err != nil {
		log.Printf("Failed to post movie comment for clerkId=%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if res.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusConflict)
		return
	}
	if !res.OK() {
		log.Printf("Failed to post movie comment for clerkId=%s: movies-svc status %d", clerkID, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) reactToMovieComment(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clerkID := clerkauth.ClerkUserID(r)

	userRes, err := h.userClient.GetCurrentUser(r.Context(), clerkID)
	if err != nil {
		log.Printf("Failed to react to movie comment for clerkId=%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if userRes.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusConflict)
		return
	}
	if !userRes.OK() {
		log.Printf("Failed to react to movie comment for clerkId=%s: user-svc status %d", clerkID, userRes.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	user, err := users.ParseUserResponse(userRes)
	if err != nil {
		log.Printf("Failed to react to movie comment for clerkId=%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	res, err := h.client.ReactToComment(r.Context(), q.Get("commentId"), q.Get("reactionType"), user.DisplayName, user.DisplayName, user.ProfileImageURL, clerkID)
	if err != nil {
		log.Printf("Failed to react to movie comment for clerkId=%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if res.StatusCode == http.StatusNotFound {
		// lumo-user-svc doesn't have this clerkId yet, or the comment itself
		// is gone — either way there's nothing to react to.
		w.WriteHeader(http.StatusConflict)
		return
	}
	if !res.OK() {
		log.Printf("Failed to react to movie comment for clerkId=%s: movies-svc status %d", clerkID, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) getMovieCommentsReactions(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.GetCommentsReactions(r.Context(), r.URL.Query().Get("commentIds"))
	proxyOr500(w, "GET /get-movie-comments-reactions", res, err)
}

// writeCommentMutationResult mirrors the shared try/catch shape in
// deleteMovieComment/updateMovieComment (and their series equivalents):
// FeignException.Forbidden -> 403, .NotFound -> 404, any other
// FeignException -> logged + 502, success -> 204.
func writeCommentMutationResult(w http.ResponseWriter, label string, res *downstream.Result, err error) {
	if err != nil {
		log.Printf("%s: %v", label, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	switch res.StatusCode {
	case http.StatusForbidden:
		w.WriteHeader(http.StatusForbidden)
		return
	case http.StatusNotFound:
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if !res.OK() {
		log.Printf("%s: downstream status %d", label, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func queryOr(q map[string][]string, key, fallback string) string {
	if v, ok := q[key]; ok && len(v) > 0 && v[0] != "" {
		return v[0]
	}
	return fallback
}

func isBlank(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON: %v", err)
	}
}

// emptyMovie mirrors `new Movie()` — every field at its Java zero value
// (null/empty-list), which Jackson serializes with castList/imagesList as
// empty arrays and every String field omitted as null.
var emptyMovie = map[string]any{
	"id":                nil,
	"title":             nil,
	"tmdbId":            nil,
	"posterImgURL":      nil,
	"trailerURL":        nil,
	"specialText":       nil,
	"searchTag":         nil,
	"genres":            nil,
	"description":       nil,
	"releaseDate":       nil,
	"tmdbRating":        nil,
	"backgroundImg_URL": nil,
	"createdAt":         nil,
	"castList":          []any{},
	"videoURL":          nil,
	"imagesList":        []any{},
}

// getMovieTitles proxies the current (translated, via ?lang) titles and
// posters for a list of TMDB ids — for the watchlist, playlists, likes and
// last-viewed, which store a copy from when each title was saved.
func (h *Handlers) getMovieTitles(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.GetMovieTitles(r.Context(), r.URL.Query().Get("ids"))
	proxyOr500(w, "GET /get-movie-titles", res, err)
}
