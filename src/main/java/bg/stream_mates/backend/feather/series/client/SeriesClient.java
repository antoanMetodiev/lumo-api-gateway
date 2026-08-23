package bg.stream_mates.backend.feather.series.client;

import bg.stream_mates.backend.commonData.dtos.Actor;
import bg.stream_mates.backend.commonData.dtos.ActorLatestWorks;
import bg.stream_mates.backend.commonData.dtos.CinemaRecordResponse;
import bg.stream_mates.backend.config.FeignConfig;
import bg.stream_mates.backend.feather.movies.models.dtos.CommentReactionsResponse;
import bg.stream_mates.backend.feather.series.dtos.CheckForNewEpRequest;
import bg.stream_mates.backend.feather.series.models.Series;
import bg.stream_mates.backend.feather.series.models.SeriesComment;
import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.*;

import java.util.List;
import java.util.Map;

//@FeignClient(name = "stream-mate-series-svc", url = "http://localhost:8082", configuration = FeignConfig.class)
@FeignClient(name = "stream-mate-series-svc", url = "https://lumo-series-svc.onrender.com", configuration = FeignConfig.class)
public interface SeriesClient {

    @GetMapping("/actor/latest-series/{imdb_id}")
    List<ActorLatestWorks> getActorLatestSeries(@PathVariable("imdb_id") String imdb_id);

    @GetMapping(value = "/getSeriesByTmdbId/{tmdbId}")
    Series getSeriesByTmdbId(@PathVariable("tmdbId") String tmdbId);

    @GetMapping("/heartbeat")
    void heartbeat();

    @GetMapping("/get-next-thirty-series")
    List<CinemaRecordResponse> getEveryThirtySeries(@RequestParam(defaultValue = "0") int page,
                                                    @RequestParam(defaultValue = "10") int size);

    @GetMapping("/get-series-details")
    Series getConcreteSeriesDetails(@RequestParam("id") String id);

    @GetMapping("/get-all-series-count")
    long getAllSeriesCount();

    @PostMapping("/search-series")
    void searchSeries(@RequestParam String title);

    @GetMapping("/get-series-by-title")
    List<Series> getSeriesByTitle(@RequestParam("title") String title);

    @GetMapping("/get-searched-series-count")
    long getSearchedSeriesCount(@RequestParam String title);

    @GetMapping("/get-series-count-by-genre")
    long findSeriesCountByGenre(@RequestParam String genres);

    @GetMapping("/get-next-twenty-series-by-genre")
    List<CinemaRecordResponse> getNextTwentySeriesByGenre(@RequestParam(defaultValue = "0") int page,
                                                          @RequestParam(defaultValue = "20") int size,
                                                          @RequestParam String receivedGenre);

    @DeleteMapping("/delete-series-comment")
    void deleteSeriesComment(@RequestParam String commentId,
                             @RequestParam String movieId,
                             @RequestHeader("X-Clerk-User-Id") String requesterId);

    @PutMapping("/update-series-comment")
    void updateSeriesComment(@RequestParam String commentId,
                             @RequestParam String movieId,
                             @RequestParam String commentText,
                             @RequestHeader("X-Clerk-User-Id") String requesterId);

    @GetMapping("/get-next-10-series-comments")
    List<SeriesComment> getNext10SeriesComments(@RequestParam int order,
                                                @RequestParam String currentCinemaRecordId);

    // X-Clerk-User-Id here is the gateway's own already-verified caller
    // identity (see SeriesController) — series-svc trusts it because
    // FeignConfig's interceptor also stamps X-Internal-Api-Key on this same
    // call, and its own GatewayAuthenticationFilter checks both.
    @PostMapping("/post-series-comment")
    void postComment(@RequestParam String authorUsername,
                     @RequestParam String authorFullName,
                     @RequestParam String authorImgURL,
                     @RequestParam String commentText,
                     @RequestParam String createdAt,
                     @RequestParam String movieId,
                     @RequestHeader("X-Clerk-User-Id") String authorId);

    @GetMapping("/search-series-matching-results/{searchTitle}")
    List<CinemaRecordResponse> searchSeriesMatchingResults(@PathVariable(value = "searchTitle") String searchTitle);

    @PostMapping(value = "/check-for-new-episodes", consumes = "application/json")
    void checkForNewEpisodes(@RequestBody CheckForNewEpRequest checkForNewEpRequest);

    // X-Clerk-User-Id here is the gateway's own already-verified caller
    // identity (see SeriesController) — series-svc trusts it the same way it
    // trusts it for postComment above.
    @PutMapping("/react-series-comment")
    void reactToComment(@RequestParam String commentId,
                        @RequestParam String reactionType,
                        @RequestParam String authorUsername,
                        @RequestParam String authorFullName,
                        @RequestParam String authorImgURL,
                        @RequestHeader("X-Clerk-User-Id") String requesterId);

    @GetMapping("/get-series-comments-reactions")
    Map<String, CommentReactionsResponse> getCommentsReactions(@RequestParam String commentIds);

    @GetMapping("/discover-series")
    List<CinemaRecordResponse> discoverSeries(@RequestParam(required = false) String genre,
                                              @RequestParam(required = false) String year,
                                              @RequestParam(required = false) String actorName,
                                              @RequestParam(required = false) String sort,
                                              @RequestParam int page,
                                              @RequestParam int size);

    @GetMapping("/discover-series-count")
    long discoverSeriesCount(@RequestParam(required = false) String genre,
                             @RequestParam(required = false) String year,
                             @RequestParam(required = false) String actorName);

    @GetMapping("/suggest-series-actors")
    List<Actor> suggestSeriesActors(@RequestParam String query);
}
