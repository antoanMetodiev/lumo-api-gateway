// Package series mirrors SeriesClient + SeriesController. client.go holds
// the typed downstream calls to lumo-series-svc; handlers.go holds the routes.
package series

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"lumo-api-gateway/internal/downstream"
)

type Client struct {
	ds      *downstream.Client
	baseURL string
}

func NewClient(ds *downstream.Client, baseURL string) *Client {
	return &Client{ds: ds, baseURL: baseURL}
}

// GetActorWorkCounts: every actor (by IMDb id) with how many series list them.
func (c *Client) GetActorWorkCounts(ctx context.Context) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/actor-work-counts", nil, nil)
}

func (c *Client) GetActorLatestSeries(ctx context.Context, imdbID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/actor/latest-series/"+url.PathEscape(imdbID), nil, nil)
}

func (c *Client) GetSeriesByTmdbId(ctx context.Context, tmdbID string) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/getSeriesByTmdbId/"+url.PathEscape(tmdbID), nil, nil)
}

// GetTrendingSeries mirrors internal/movies.Client.GetTrendingMovies —
// lumo-series-svc grew its own trending endpoints to match movies-svc's.
func (c *Client) GetTrendingSeries(ctx context.Context) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/get-trending-series", nil, nil)
}

func (c *Client) Heartbeat(ctx context.Context) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/heartbeat", nil, nil)
}

func (c *Client) GetEveryThirtySeries(ctx context.Context, page, size string) (*downstream.Result, error) {
	q := url.Values{"page": {page}, "size": {size}}
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/get-next-thirty-series?"+q.Encode(), nil, nil)
}

// GetSeriesFacts fetches lumo-series-svc's facts panel for a TMDB id.
func (c *Client) GetSeriesFacts(ctx context.Context, tmdbID string) (*downstream.Result, error) {
	q := url.Values{"tmdbId": {tmdbID}}
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/get-series-facts?"+q.Encode(), nil, nil)
}

// GetSeriesTitles fetches lumo-series-svc's titles / posters for TMDB ids.
func (c *Client) GetSeriesTitles(ctx context.Context, ids string) (*downstream.Result, error) {
	q := url.Values{"ids": {ids}}
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/get-series-titles?"+q.Encode(), nil, nil)
}

func (c *Client) GetConcreteSeriesDetails(ctx context.Context, id string) (*downstream.Result, error) {
	q := url.Values{"id": {id}}
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/get-series-details?"+q.Encode(), nil, nil)
}

func (c *Client) GetAllSeriesCount(ctx context.Context) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/get-all-series-count", nil, nil)
}

// SearchSeries mirrors SeriesClient.searchSeries — unlike movies' equivalent
// (a separate generator service taking a JSON body), this hits lumo-series-svc
// itself with the title as a query param.
func (c *Client) SearchSeries(ctx context.Context, title string) (*downstream.Result, error) {
	q := url.Values{"title": {title}}
	return c.ds.Do(ctx, http.MethodPost, c.baseURL+"/search-series?"+q.Encode(), nil, nil)
}

func (c *Client) GetSeriesByTitle(ctx context.Context, title string) (*downstream.Result, error) {
	q := url.Values{"title": {title}}
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/get-series-by-title?"+q.Encode(), nil, nil)
}

func (c *Client) GetSearchedSeriesCount(ctx context.Context, title string) (*downstream.Result, error) {
	q := url.Values{"title": {title}}
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/get-searched-series-count?"+q.Encode(), nil, nil)
}

// FindSeriesCountByGenre mirrors SeriesClient.findSeriesCountByGenre — the
// Feign query param is "genres", not "receivedGenre" (the gateway's own
// incoming param name); see SeriesController.findMoviesCountByGenre.
func (c *Client) FindSeriesCountByGenre(ctx context.Context, genres string) (*downstream.Result, error) {
	q := url.Values{"genres": {genres}}
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/get-series-count-by-genre?"+q.Encode(), nil, nil)
}

func (c *Client) GetNextTwentySeriesByGenre(ctx context.Context, page, size, receivedGenre string) (*downstream.Result, error) {
	q := url.Values{"page": {page}, "size": {size}, "receivedGenre": {receivedGenre}}
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/get-next-twenty-series-by-genre?"+q.Encode(), nil, nil)
}

func (c *Client) DeleteSeriesComment(ctx context.Context, commentID, movieID, requesterID string) (*downstream.Result, error) {
	q := url.Values{"commentId": {commentID}, "movieId": {movieID}}
	u := c.baseURL + "/delete-series-comment?" + q.Encode()
	return c.ds.Do(ctx, http.MethodDelete, u, nil, map[string]string{"X-Clerk-User-Id": requesterID})
}

func (c *Client) UpdateSeriesComment(ctx context.Context, commentID, movieID, commentText, requesterID string) (*downstream.Result, error) {
	q := url.Values{"commentId": {commentID}, "movieId": {movieID}, "commentText": {commentText}}
	u := c.baseURL + "/update-series-comment?" + q.Encode()
	return c.ds.Do(ctx, http.MethodPut, u, nil, map[string]string{"X-Clerk-User-Id": requesterID})
}

func (c *Client) GetNext10SeriesComments(ctx context.Context, order, currentCinemaRecordID string) (*downstream.Result, error) {
	q := url.Values{"order": {order}, "currentCinemaRecordId": {currentCinemaRecordID}}
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/get-next-10-series-comments?"+q.Encode(), nil, nil)
}

// PostComment mirrors SeriesClient.postComment — see movies.Client.PostComment's
// comment about X-Clerk-User-Id.
func (c *Client) PostComment(ctx context.Context, authorUsername, authorFullName, authorImgURL, commentText, createdAt, movieID, authorID string) (*downstream.Result, error) {
	q := url.Values{
		"authorUsername": {authorUsername},
		"authorFullName": {authorFullName},
		"authorImgURL":   {authorImgURL},
		"commentText":    {commentText},
		"createdAt":      {createdAt},
		"movieId":        {movieID},
	}
	u := c.baseURL + "/post-series-comment?" + q.Encode()
	return c.ds.Do(ctx, http.MethodPost, u, nil, map[string]string{"X-Clerk-User-Id": authorID})
}

func (c *Client) SearchSeriesMatchingResults(ctx context.Context, searchTitle string, limit, offset int) (*downstream.Result, error) {
	q := url.Values{"limit": {strconv.Itoa(limit)}, "offset": {strconv.Itoa(offset)}}
	u := c.baseURL + "/search-series-matching-results/" + url.PathEscape(searchTitle) + "?" + q.Encode()
	return c.ds.Do(ctx, http.MethodGet, u, nil, nil)
}

// CheckForNewEpisodes mirrors SeriesClient.checkForNewEpisodes — the JSON
// body is forwarded exactly as the client sent it (no @Valid on this one in
// Java, so nothing to validate first).
func (c *Client) CheckForNewEpisodes(ctx context.Context, body []byte) (*downstream.Result, error) {
	return c.ds.Do(ctx, http.MethodPost, c.baseURL+"/check-for-new-episodes", body, nil)
}

func (c *Client) ReactToComment(ctx context.Context, commentID, reactionType, authorUsername, authorFullName, authorImgURL, requesterID string) (*downstream.Result, error) {
	q := url.Values{
		"commentId":      {commentID},
		"reactionType":   {reactionType},
		"authorUsername": {authorUsername},
		"authorFullName": {authorFullName},
		"authorImgURL":   {authorImgURL},
	}
	u := c.baseURL + "/react-series-comment?" + q.Encode()
	return c.ds.Do(ctx, http.MethodPut, u, nil, map[string]string{"X-Clerk-User-Id": requesterID})
}

func (c *Client) GetCommentsReactions(ctx context.Context, commentIDs string) (*downstream.Result, error) {
	q := url.Values{"commentIds": {commentIDs}}
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/get-series-comments-reactions?"+q.Encode(), nil, nil)
}

func (c *Client) DiscoverSeries(ctx context.Context, genre, year, actorName, network, sort, page, size string) (*downstream.Result, error) {
	q := url.Values{"page": {page}, "size": {size}}
	setIfPresent(q, "genre", genre)
	setIfPresent(q, "year", year)
	setIfPresent(q, "actorName", actorName)
	setIfPresent(q, "network", network)
	setIfPresent(q, "sort", sort)
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/discover-series?"+q.Encode(), nil, nil)
}

func (c *Client) DiscoverSeriesCount(ctx context.Context, genre, year, actorName, network string) (*downstream.Result, error) {
	q := url.Values{}
	setIfPresent(q, "genre", genre)
	setIfPresent(q, "year", year)
	setIfPresent(q, "actorName", actorName)
	setIfPresent(q, "network", network)
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/discover-series-count?"+q.Encode(), nil, nil)
}

func (c *Client) SuggestSeriesActors(ctx context.Context, query string) (*downstream.Result, error) {
	q := url.Values{"query": {query}}
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/suggest-series-actors?"+q.Encode(), nil, nil)
}

func setIfPresent(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

// GetActor fetches one actor by IMDb id (translated on ?lang).
func (c *Client) GetActor(ctx context.Context, imdbID string) (*downstream.Result, error) {
	q := url.Values{"imdbId": {imdbID}}
	return c.ds.Do(ctx, http.MethodGet, c.baseURL+"/get-actor?"+q.Encode(), nil, nil)
}
