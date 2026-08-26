package series

import (
	"encoding/json"
	"io"
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

// Handlers mirrors SeriesController — same paths, same params, same status
// codes.
type Handlers struct {
	client     *Client
	userClient *users.Client
}

func NewHandlers(client *Client, userClient *users.Client) *Handlers {
	return &Handlers{client: client, userClient: userClient}
}

func (h *Handlers) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /getSeriesByTmdbId/{tmdbId}", h.getSeriesByTmdbId)
	mux.HandleFunc("POST /check-for-new-episodes", h.checkForNewEpisodes)
	mux.HandleFunc("GET /search-series-matching-results/{searchTitle}", h.searchSeriesMatchingResults)
	mux.HandleFunc("DELETE /delete-series-comment", h.deleteSeriesComment)
	mux.HandleFunc("PUT /update-series-comment", h.updateSeriesComment)
	mux.HandleFunc("GET /get-next-10-series-comments", h.getNext10SeriesComments)
	mux.HandleFunc("POST /post-series-comment", h.postComment)
	mux.HandleFunc("GET /get-series-count-by-genre", h.findSeriesCountByGenre)
	mux.HandleFunc("GET /get-next-twenty-series-by-genre", h.getNextTwentySeriesByGenre)
	mux.HandleFunc("GET /get-searched-series-count", h.getSearchedSeriesCount)
	mux.HandleFunc("GET /get-series-by-title", h.getSeriesByTitle)
	mux.HandleFunc("GET /get-next-thirty-series", h.getEveryThirtySeries)
	mux.HandleFunc("GET /get-series-details", h.getConcreteSeriesDetails)
	mux.HandleFunc("GET /get-all-series-count", h.getAllSeriesCount)
	mux.HandleFunc("POST /search-series", h.searchSeries)
	mux.HandleFunc("PUT /react-series-comment", h.reactToSeriesComment)
	mux.HandleFunc("GET /get-series-comments-reactions", h.getSeriesCommentsReactions)
	mux.HandleFunc("GET /discover-series", h.discoverSeries)
	mux.HandleFunc("GET /discover-series-count", h.discoverSeriesCount)
	mux.HandleFunc("GET /suggest-series-actors", h.suggestSeriesActors)
}

// proxyOr500 mirrors movies.proxyOr500 — no try/catch in the Java
// controller, so any downstream failure falls through to Spring's default
// unhandled-exception behavior (500).
func proxyOr500(w http.ResponseWriter, label string, res *downstream.Result, err error) {
	if err != nil {
		log.Printf("%s: %v", label, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !res.OK() {
		log.Printf("%s: downstream status %d", label, res.StatusCode)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	downstream.WriteProxy(w, res)
}

func (h *Handlers) getSeriesByTmdbId(w http.ResponseWriter, r *http.Request) {
	tmdbID := r.PathValue("tmdbId")
	res, err := h.client.GetSeriesByTmdbId(r.Context(), tmdbID)
	proxyOr500(w, "GET /getSeriesByTmdbId", res, err)
}

// checkForNewEpisodes mirrors SeriesController.checkForNewEpisodes: catches
// any error and returns 500 (rather than letting it propagate to Spring's
// default handling like most of this controller does).
func (h *Handlers) checkForNewEpisodes(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	res, err := h.client.CheckForNewEpisodes(r.Context(), body)
	if err != nil || !res.OK() {
		if err != nil {
			log.Printf("POST /check-for-new-episodes: %v", err)
		} else {
			log.Printf("POST /check-for-new-episodes: downstream status %d", res.StatusCode)
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) searchSeriesMatchingResults(w http.ResponseWriter, r *http.Request) {
	searchTitle := r.PathValue("searchTitle")
	limit := queryInt(r, "limit", 10)
	offset := queryInt(r, "offset", 0)
	res, err := h.client.SearchSeriesMatchingResults(r.Context(), searchTitle, limit, offset)
	proxyOr500(w, "GET /search-series-matching-results", res, err)
}

func (h *Handlers) deleteSeriesComment(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	requesterID := clerkauth.ClerkUserID(r)
	res, err := h.client.DeleteSeriesComment(r.Context(), q.Get("commentId"), q.Get("movieId"), requesterID)
	writeCommentMutationResult(w, "DELETE /delete-series-comment", res, err)
}

func (h *Handlers) updateSeriesComment(w http.ResponseWriter, r *http.Request) {
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

	res, err := h.client.UpdateSeriesComment(r.Context(), q.Get("commentId"), q.Get("movieId"), req.CommentText, requesterID)
	writeCommentMutationResult(w, "PUT /update-series-comment", res, err)
}

func (h *Handlers) getNext10SeriesComments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := h.client.GetNext10SeriesComments(r.Context(), q.Get("order"), q.Get("currentCinemaRecordId"))
	proxyOr500(w, "GET /get-next-10-series-comments", res, err)
}

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
		log.Printf("Failed to post series comment for clerkId=%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if userRes.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusConflict)
		return
	}
	if !userRes.OK() {
		log.Printf("Failed to post series comment for clerkId=%s: user-svc status %d", clerkID, userRes.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	user, err := users.ParseUserResponse(userRes)
	if err != nil {
		log.Printf("Failed to post series comment for clerkId=%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := h.client.PostComment(r.Context(), user.DisplayName, user.DisplayName, user.ProfileImageURL, req.CommentText, createdAt, req.CinemaRecordID, clerkID)
	if err != nil {
		log.Printf("Failed to post series comment for clerkId=%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if res.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusConflict)
		return
	}
	if !res.OK() {
		log.Printf("Failed to post series comment for clerkId=%s: series-svc status %d", clerkID, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) findSeriesCountByGenre(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.FindSeriesCountByGenre(r.Context(), r.URL.Query().Get("receivedGenre"))
	proxyOr500(w, "GET /get-series-count-by-genre", res, err)
}

func (h *Handlers) getNextTwentySeriesByGenre(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := queryOr(q, "page", "0")
	size := queryOr(q, "size", "20")
	res, err := h.client.GetNextTwentySeriesByGenre(r.Context(), page, size, q.Get("receivedGenre"))
	proxyOr500(w, "GET /get-next-twenty-series-by-genre", res, err)
}

func (h *Handlers) getSearchedSeriesCount(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.GetSearchedSeriesCount(r.Context(), r.URL.Query().Get("title"))
	proxyOr500(w, "GET /get-searched-series-count", res, err)
}

func (h *Handlers) getSeriesByTitle(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.GetSeriesByTitle(r.Context(), r.URL.Query().Get("title"))
	proxyOr500(w, "GET /get-series-by-title", res, err)
}

func (h *Handlers) getEveryThirtySeries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := queryOr(q, "page", "0")
	size := queryOr(q, "size", "10")
	res, err := h.client.GetEveryThirtySeries(r.Context(), page, size)
	proxyOr500(w, "GET /get-next-thirty-series", res, err)
}

func (h *Handlers) getConcreteSeriesDetails(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.GetConcreteSeriesDetails(r.Context(), r.URL.Query().Get("id"))
	proxyOr500(w, "GET /get-series-details", res, err)
}

func (h *Handlers) getAllSeriesCount(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.GetAllSeriesCount(r.Context())
	proxyOr500(w, "GET /get-all-series-count", res, err)
}

// searchSeries mirrors SeriesController.searchSeries — unlike movies'
// searchMovies, there's no @Valid here, so no blank-check before forwarding.
func (h *Handlers) searchSeries(w http.ResponseWriter, r *http.Request) {
	var req models.CinemaRecRequestDto
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	res, err := h.client.SearchSeries(r.Context(), req.RecordName)
	if err != nil {
		log.Printf("POST /search-series: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !res.OK() {
		log.Printf("POST /search-series: downstream status %d", res.StatusCode)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) reactToSeriesComment(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	clerkID := clerkauth.ClerkUserID(r)

	userRes, err := h.userClient.GetCurrentUser(r.Context(), clerkID)
	if err != nil {
		log.Printf("Failed to react to series comment for clerkId=%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if userRes.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusConflict)
		return
	}
	if !userRes.OK() {
		log.Printf("Failed to react to series comment for clerkId=%s: user-svc status %d", clerkID, userRes.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	user, err := users.ParseUserResponse(userRes)
	if err != nil {
		log.Printf("Failed to react to series comment for clerkId=%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	res, err := h.client.ReactToComment(r.Context(), q.Get("commentId"), q.Get("reactionType"), user.DisplayName, user.DisplayName, user.ProfileImageURL, clerkID)
	if err != nil {
		log.Printf("Failed to react to series comment for clerkId=%s: %v", clerkID, err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	if res.StatusCode == http.StatusNotFound {
		w.WriteHeader(http.StatusConflict)
		return
	}
	if !res.OK() {
		log.Printf("Failed to react to series comment for clerkId=%s: series-svc status %d", clerkID, res.StatusCode)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) getSeriesCommentsReactions(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.GetCommentsReactions(r.Context(), r.URL.Query().Get("commentIds"))
	proxyOr500(w, "GET /get-series-comments-reactions", res, err)
}

func (h *Handlers) discoverSeries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page := queryOr(q, "page", "0")
	size := queryOr(q, "size", "30")
	res, err := h.client.DiscoverSeries(r.Context(), q.Get("genre"), q.Get("year"), q.Get("actorName"), q.Get("sort"), page, size)
	proxyOr500(w, "GET /discover-series", res, err)
}

func (h *Handlers) discoverSeriesCount(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := h.client.DiscoverSeriesCount(r.Context(), q.Get("genre"), q.Get("year"), q.Get("actorName"))
	proxyOr500(w, "GET /discover-series-count", res, err)
}

func (h *Handlers) suggestSeriesActors(w http.ResponseWriter, r *http.Request) {
	res, err := h.client.SuggestSeriesActors(r.Context(), r.URL.Query().Get("query"))
	proxyOr500(w, "GET /suggest-series-actors", res, err)
}

// writeCommentMutationResult mirrors movies.writeCommentMutationResult — see
// its comment.
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
