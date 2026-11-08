// Package movies mirrors MovieClient + MovieController + MovieCommentsController
// + TrendingMoviesController. client.go holds the typed downstream calls
// (to lumo-movies-svc, lumo-movies-generator, lumo-movies-generator-by-actors);
// handlers.go holds the routes.
package movies

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"lumo-api-gateway/internal/downstream"
)

type Client struct {
	ds                         *downstream.Client
	moviesSvcURL               string
	moviesGeneratorURL         string
	moviesGeneratorByActorsURL string
}

func NewClient(ds *downstream.Client, moviesSvcURL, moviesGeneratorURL, moviesGeneratorByActorsURL string) *Client {
	return &Client{
		ds:                         ds,
		moviesSvcURL:               moviesSvcURL,
		moviesGeneratorURL:         moviesGeneratorURL,
		moviesGeneratorByActorsURL: moviesGeneratorByActorsURL,
	}
}

func (c *Client) GetActorLatestMovies(ctx context.Context, imdbID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/actor/latest-movies/"+url.PathEscape(imdbID), nil, nil)
}

func (c *Client) GetTrendingMovies(ctx context.Context) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/get-trending-movies", nil, nil)
}

func (c *Client) Heartbeat(ctx context.Context) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/heartbeat", nil, nil)
}

// PostComment mirrors MovieClient.postComment. X-Clerk-User-Id is the
// gateway's own already-verified caller identity — movies-svc trusts it
// because downstream.Client also stamps X-Internal-Api-Key on this same
// call, and its own gateway-trust check verifies both.
func (c *Client) PostComment(ctx context.Context, authorUsername, authorFullName, authorImgURL, commentText, createdAt, movieID, authorID string) (*downstream.Result, error) {
	q := url.Values{
		"authorUsername": {authorUsername},
		"authorFullName": {authorFullName},
		"authorImgURL":   {authorImgURL},
		"commentText":    {commentText},
		"createdAt":      {createdAt},
		"movieId":        {movieID},
	}
	u := c.moviesSvcURL + "/post-movie-comment?" + q.Encode()
	return c.ds.Do(ctx, http.MethodPost, u, nil, map[string]string{"X-Clerk-User-Id": authorID})
}

func (c *Client) UpdateMovieComment(ctx context.Context, commentID, movieID, commentText, requesterID string) (*downstream.Result, error) {
	q := url.Values{"commentId": {commentID}, "movieId": {movieID}, "commentText": {commentText}}
	u := c.moviesSvcURL + "/update-movie-comment?" + q.Encode()
	return c.ds.Do(ctx, http.MethodPut, u, nil, map[string]string{"X-Clerk-User-Id": requesterID})
}

func (c *Client) GetEveryThirtyMovies(ctx context.Context, page, size string) (*downstream.Result, error) {
	q := url.Values{"page": {page}, "size": {size}}
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/get-next-thirty-movies?"+q.Encode(), nil, nil)
}

func (c *Client) GetMoviesByTitle(ctx context.Context, title string) (*downstream.Result, error) {
	q := url.Values{"title": {title}}
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/get-movies-by-title?"+q.Encode(), nil, nil)
}

func (c *Client) GetConcreteMovieDetails(ctx context.Context, movieID string) (*downstream.Result, error) {
	q := url.Values{"movieId": {movieID}}
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/get-movie-details?"+q.Encode(), nil, nil)
}

// GetMovieFacts fetches lumo-movies-svc's facts panel for a TMDB id.
func (c *Client) GetMovieFacts(ctx context.Context, tmdbID string) (*downstream.Result, error) {
	q := url.Values{"tmdbId": {tmdbID}}
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/get-movie-facts?"+q.Encode(), nil, nil)
}

// GetMovieTitles fetches lumo-movies-svc's titles / posters for TMDB ids.
func (c *Client) GetMovieTitles(ctx context.Context, ids string) (*downstream.Result, error) {
	q := url.Values{"ids": {ids}}
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/get-movie-titles?"+q.Encode(), nil, nil)
}

// SearchMovies mirrors MoviesGeneratorClient.searchMovies — Feign's
// JacksonEncoder encodes even a bare String request body as a JSON string
// literal (quoted), so that's what's forwarded here too.
func (c *Client) SearchMovies(ctx context.Context, title string) (*downstream.Result, error) {
	body, _ := json.Marshal(title)
	return c.ds.Do(ctx, http.MethodPost, c.moviesGeneratorURL+"/search-movies", body, nil)
}

func (c *Client) GetAllMoviesCount(ctx context.Context) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/get-all-movies-count", nil, nil)
}

func (c *Client) GetNextTwentyMoviesByGenre(ctx context.Context, page, size, receivedGenre string) (*downstream.Result, error) {
	q := url.Values{"page": {page}, "size": {size}, "receivedGenre": {receivedGenre}}
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/get-next-twenty-movies-by-genre?"+q.Encode(), nil, nil)
}

// FindMoviesCountByGenre mirrors MovieClient.findMoviesCountByGenre — the
// Feign query param is "genres", not "receivedGenre" (the gateway's own
// incoming param name); see MovieController.findMoviesCountByGenre.
func (c *Client) FindMoviesCountByGenre(ctx context.Context, genres string) (*downstream.Result, error) {
	q := url.Values{"genres": {genres}}
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/get-movies-count-by-genre?"+q.Encode(), nil, nil)
}

func (c *Client) GetSearchedMoviesCount(ctx context.Context, title string) (*downstream.Result, error) {
	q := url.Values{"title": {title}}
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/get-searched-movies-count?"+q.Encode(), nil, nil)
}

func (c *Client) GetNext10Comments(ctx context.Context, order, currentCinemaRecordID string) (*downstream.Result, error) {
	q := url.Values{"order": {order}, "currentCinemaRecordId": {currentCinemaRecordID}}
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/get-next-10-movie-comments?"+q.Encode(), nil, nil)
}

func (c *Client) DeleteMovieComment(ctx context.Context, commentID, movieID, requesterID string) (*downstream.Result, error) {
	q := url.Values{"commentId": {commentID}, "movieId": {movieID}}
	u := c.moviesSvcURL + "/delete-movie-comment?" + q.Encode()
	return c.ds.Do(ctx, http.MethodDelete, u, nil, map[string]string{"X-Clerk-User-Id": requesterID})
}

func (c *Client) SearchMoviesMatchingResults(ctx context.Context, searchTitle string, limit, offset int) (*downstream.Result, error) {
	q := url.Values{"limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)}}
	u := c.moviesSvcURL + "/search-movies-matching-results/" + url.PathEscape(searchTitle) + "?" + q.Encode()
	return c.ds.Do(ctx, http.MethodGet, u, nil, nil)
}

func (c *Client) AddMoviesByActor(ctx context.Context, imdbID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodPost, c.moviesGeneratorByActorsURL+"/add-movies-by-actor/"+url.PathEscape(imdbID), nil, nil)
}

// ReactToComment mirrors MovieClient.reactToComment — see PostComment's
// comment about X-Clerk-User-Id.
func (c *Client) ReactToComment(ctx context.Context, commentID, reactionType, authorUsername, authorFullName, authorImgURL, requesterID string) (*downstream.Result, error) {
	q := url.Values{
		"commentId":      {commentID},
		"reactionType":   {reactionType},
		"authorUsername": {authorUsername},
		"authorFullName": {authorFullName},
		"authorImgURL":   {authorImgURL},
	}
	u := c.moviesSvcURL + "/react-movie-comment?" + q.Encode()
	return c.ds.Do(ctx, http.MethodPut, u, nil, map[string]string{"X-Clerk-User-Id": requesterID})
}

func (c *Client) GetCommentsReactions(ctx context.Context, commentIDs string) (*downstream.Result, error) {
	q := url.Values{"commentIds": {commentIDs}}
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/get-movie-comments-reactions?"+q.Encode(), nil, nil)
}

// DiscoverMovies mirrors MovieClient.discoverMovies — genre/year/actorName/sort
// are Feign @RequestParam(required=false): an empty value here is simply
// omitted from the outgoing query string, same as Feign never sending a null
// param. audio ("bg") keeps only the movies with a Bulgarian dub.
func (c *Client) DiscoverMovies(ctx context.Context, genre, year, actorName, sort, audio, page, size string) (*downstream.Result, error) {
	q := url.Values{"page": {page}, "size": {size}}
	setIfPresent(q, "genre", genre)
	setIfPresent(q, "year", year)
	setIfPresent(q, "actorName", actorName)
	setIfPresent(q, "sort", sort)
	setIfPresent(q, "audio", audio)
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/discover-movies?"+q.Encode(), nil, nil)
}

func (c *Client) DiscoverMoviesCount(ctx context.Context, genre, year, actorName, audio string) (*downstream.Result, error) {
	q := url.Values{}
	setIfPresent(q, "genre", genre)
	setIfPresent(q, "year", year)
	setIfPresent(q, "actorName", actorName)
	setIfPresent(q, "audio", audio)
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/discover-movies-count?"+q.Encode(), nil, nil)
}

func (c *Client) SuggestMovieActors(ctx context.Context, query string) (*downstream.Result, error) {
	q := url.Values{"query": {query}}
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/suggest-movie-actors?"+q.Encode(), nil, nil)
}

func setIfPresent(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

// GetActor fetches one actor by IMDb id (translated on ?lang).
func (c *Client) GetActor(ctx context.Context, imdbID string) (*downstream.Result, error) {
	q := url.Values{"imdbId": {imdbID}}
	return c.ds.Do(ctx, http.MethodGet, c.moviesSvcURL+"/get-actor?"+q.Encode(), nil, nil)
}
