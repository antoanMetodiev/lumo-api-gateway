package bg.stream_mates.backend.feather.movies.client;

import bg.stream_mates.backend.commonData.dtos.Actor;
import bg.stream_mates.backend.commonData.dtos.ActorLatestWorks;
import bg.stream_mates.backend.commonData.dtos.CinemaRecordResponse;
import bg.stream_mates.backend.config.FeignConfig;
import bg.stream_mates.backend.feather.movies.models.dtos.CommentReactionsResponse;
import bg.stream_mates.backend.feather.movies.models.dtos.MovieComment;
import bg.stream_mates.backend.feather.movies.models.entities.Movie;
import bg.stream_mates.backend.feather.movies.models.entities.TrendingMovie;
import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.*;

import java.util.List;
import java.util.Map;

//@FeignClient(name = "stream-mate-movie-svc", u/rl = "http://localhost:8081", configuration = FeignConfig.class)
@FeignClient(name = "stream-mate-movie-svc", url = "https://lumo-movies-svc.onrender.com", configuration = FeignConfig.class)
public interface MovieClient {

    @GetMapping("/actor/latest-movies/{imdb_id}")
    List<ActorLatestWorks> getActorLatestMovies(@PathVariable(name = "imdb_id") String imdb_id);

    @GetMapping("/get-trending-movies")
    List<TrendingMovie> getTrendingMovies();

    @GetMapping("/heartbeat")
    void heartbeat();

    // X-Clerk-User-Id here is the gateway's own already-verified caller
    // identity (see MovieCommentsController) — movies-svc trusts it because
    // FeignConfig's interceptor also stamps X-Internal-Api-Key on this same
    // call, and its own GatewayAuthenticationFilter checks both.
    @PostMapping("/post-movie-comment")
    void postComment(
            @RequestParam String authorUsername,
            @RequestParam String authorFullName,
            @RequestParam String authorImgURL,
            @RequestParam String commentText,
            @RequestParam String createdAt,
            @RequestParam String movieId,
            @RequestHeader("X-Clerk-User-Id") String authorId);

    @PutMapping("/update-movie-comment")
    void updateMovieComment(
            @RequestParam String commentId,
            @RequestParam String movieId,
            @RequestParam String commentText,
            @RequestHeader("X-Clerk-User-Id") String requesterId);


    @GetMapping("/get-next-thirty-movies")
    List<CinemaRecordResponse> getEveryThirtyMovies(@RequestParam(defaultValue = "0") int page,
                                                    @RequestParam(defaultValue = "10") int size);

    @GetMapping("/get-movies-by-title")
    List<Movie> getMoviesByTitle(@RequestParam String title);

    @GetMapping("/get-movie-details")
    Movie getConcreteMovieDetails(@RequestParam String movieId);

    @PostMapping("/search-movies")
    void searchMovies(@RequestBody String title);

    @GetMapping("/get-all-movies-count")
    long getAllMoviesCount();

    @GetMapping("/get-next-twenty-movies-by-genre")
    List<CinemaRecordResponse> getNextTwentyMoviesByGenre(@RequestParam(defaultValue = "0") int page,
                                                          @RequestParam(defaultValue = "20") int size,
                                                          @RequestParam String receivedGenre);

    @GetMapping("/get-movies-count-by-genre")
    long findMoviesCountByGenre(@RequestParam String genres);

    @GetMapping("/get-searched-movies-count")
    long getSearchedMoviesCount(@RequestParam String title);

    @GetMapping("/get-next-10-movie-comments")
    List<MovieComment> getNext10Comments(@RequestParam int order, @RequestParam String currentCinemaRecordId);

    @DeleteMapping("/delete-movie-comment")
    void deleteMovieComment(@RequestParam String commentId,
                            @RequestParam String movieId,
                            @RequestHeader("X-Clerk-User-Id") String requesterId);

    @GetMapping("/search-movies-matching-results/{searchTitle}")
    List<CinemaRecordResponse> searchMoviesMatchingResults(@PathVariable(value = "searchTitle") String searchTitle);

    @PostMapping("/add-movies-by-actor/{imdb_id}")
    void addMoviesByActor(@PathVariable(name = "imdb_id") String imdb_id);

    // X-Clerk-User-Id here is the gateway's own already-verified caller
    // identity (see MovieCommentsController) — movies-svc trusts it the same
    // way it trusts it for postComment above.
    @PutMapping("/react-movie-comment")
    void reactToComment(@RequestParam String commentId,
                        @RequestParam String reactionType,
                        @RequestParam String authorUsername,
                        @RequestParam String authorFullName,
                        @RequestParam String authorImgURL,
                        @RequestHeader("X-Clerk-User-Id") String requesterId);

    @GetMapping("/get-movie-comments-reactions")
    Map<String, CommentReactionsResponse> getCommentsReactions(@RequestParam String commentIds);

    @GetMapping("/discover-movies")
    List<CinemaRecordResponse> discoverMovies(@RequestParam(required = false) String genre,
                                              @RequestParam(required = false) String year,
                                              @RequestParam(required = false) String actorName,
                                              @RequestParam(required = false) String sort,
                                              @RequestParam int page,
                                              @RequestParam int size);

    @GetMapping("/discover-movies-count")
    long discoverMoviesCount(@RequestParam(required = false) String genre,
                             @RequestParam(required = false) String year,
                             @RequestParam(required = false) String actorName);

    @GetMapping("/suggest-movie-actors")
    List<Actor> suggestMovieActors(@RequestParam String query);
}
