package bg.stream_mates.backend.feather.series.controllers;

import bg.stream_mates.backend.commonData.dtos.Actor;
import bg.stream_mates.backend.commonData.dtos.CinemaRecRequestDto;
import bg.stream_mates.backend.commonData.dtos.CinemaRecordResponse;
import bg.stream_mates.backend.feather.movies.models.dtos.CommentReactionsResponse;
import bg.stream_mates.backend.feather.movies.models.dtos.PostCommentRequest;
import bg.stream_mates.backend.feather.movies.models.dtos.UpdateCommentRequest;
import bg.stream_mates.backend.feather.series.client.SeriesClient;

import bg.stream_mates.backend.feather.series.dtos.CheckForNewEpRequest;
import bg.stream_mates.backend.feather.series.models.Series;
import bg.stream_mates.backend.feather.series.models.SeriesComment;
import bg.stream_mates.backend.feather.users.client.UserClient;
import bg.stream_mates.backend.feather.users.dtos.UserResponse;
import feign.FeignException;
import jakarta.validation.Valid;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;

import java.io.IOException;
import java.time.Instant;
import java.util.List;
import java.util.Map;

/**
 * Reading comments (GET) is public. Posting/updating/deleting requires a
 * valid Clerk session — see SecurityConfig.PROTECTED_PATHS — and always uses
 * the already-verified caller identity (authentication.getName()), never
 * anything the client body claims to be. Display name/avatar come from
 * lumo-user-svc's own record (via UserClient), not client input either —
 * this used to let anyone post a comment as anyone else.
 */
@Slf4j
@RestController
public class SeriesController {

    private final SeriesClient seriesClient;
    private final UserClient userClient;

    @Autowired
    public SeriesController(SeriesClient seriesClient, UserClient userClient) {
        this.seriesClient = seriesClient;
        this.userClient = userClient;
    }

    @GetMapping("/getSeriesByTmdbId/{tmdbId}")
    public Series getSeriesByTmdbId(@PathVariable("tmdbId") String tmdbId) {
        return this.seriesClient.getSeriesByTmdbId(tmdbId);
    }

    @PostMapping("/check-for-new-episodes")
    public ResponseEntity<Void> checkForNewEpisodes(@RequestBody CheckForNewEpRequest checkForNewEpRequest) {

        try {
            // code
            seriesClient.checkForNewEpisodes(checkForNewEpRequest);
            return ResponseEntity.ok().build();
        } catch (Exception exception) {
            exception.printStackTrace();
            return ResponseEntity.internalServerError().build();
        }
    }

    @GetMapping("/search-series-matching-results/{searchTitle}")
    public ResponseEntity<List<CinemaRecordResponse>> searchSeriesMatchingResults(@PathVariable(value = "searchTitle") String searchTitle) {

        List<CinemaRecordResponse> response = seriesClient.searchSeriesMatchingResults(searchTitle);
        return ResponseEntity.ok(response);
    }

    @DeleteMapping("/delete-series-comment")
    public ResponseEntity<Void> deleteSeriesComment(@RequestParam String commentId,
                                                    @RequestParam String movieId,
                                                    Authentication authentication) {
        try {
            seriesClient.deleteSeriesComment(commentId, movieId, authentication.getName());
            return ResponseEntity.noContent().build();
        } catch (FeignException.Forbidden e) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).build();
        } catch (FeignException.NotFound e) {
            return ResponseEntity.notFound().build();
        } catch (FeignException e) {
            log.error("lumo-series-svc call failed for DELETE /delete-series-comment", e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @PutMapping("/update-series-comment")
    public ResponseEntity<Void> updateSeriesComment(@RequestParam String commentId,
                                                    @RequestParam String movieId,
                                                    @RequestBody @Valid UpdateCommentRequest request,
                                                    Authentication authentication) {
        try {
            seriesClient.updateSeriesComment(commentId, movieId, request.getCommentText(), authentication.getName());
            return ResponseEntity.noContent().build();
        } catch (FeignException.Forbidden e) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).build();
        } catch (FeignException.NotFound e) {
            return ResponseEntity.notFound().build();
        } catch (FeignException e) {
            log.error("lumo-series-svc call failed for PUT /update-series-comment", e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @GetMapping("/get-next-10-series-comments")
    public List<SeriesComment> getNext10SeriesComments(@RequestParam int order,
                                                       @RequestParam String currentCinemaRecordId) {

        return this.seriesClient.getNext10SeriesComments(order, currentCinemaRecordId);
    }

    @PostMapping("/post-series-comment")
    public ResponseEntity<Void> postComment(@RequestBody @Valid PostCommentRequest postCommentRequest, Authentication authentication) {
        String clerkId = authentication.getName();

        try {
            UserResponse user = userClient.getCurrentUser(clerkId);

            seriesClient.postComment(
                    user.getDisplayName(),
                    user.getDisplayName(),
                    user.getProfileImageURL(),
                    postCommentRequest.getCommentText(),
                    Instant.now().toString(),
                    postCommentRequest.getCinemaRecordId(),
                    clerkId
            );
            return ResponseEntity.noContent().build();
        } catch (FeignException.NotFound e) {
            // lumo-user-svc doesn't have this clerkId yet — the Clerk
            // webhook that syncs new accounts can race a brand new signup.
            return ResponseEntity.status(HttpStatus.CONFLICT).build();
        } catch (FeignException e) {
            log.error("Failed to post series comment for clerkId={}", clerkId, e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }


    @GetMapping("/get-series-count-by-genre")
    public long findMoviesCountByGenre(@RequestParam String receivedGenre) {
        String genres = receivedGenre;
        return this.seriesClient.findSeriesCountByGenre(receivedGenre);
    }

    @GetMapping("/get-next-twenty-series-by-genre")
    public List<CinemaRecordResponse> getNextTwentySeriesByGenre(@RequestParam(defaultValue = "0") int page,
                                                                 @RequestParam(defaultValue = "20") int size,
                                                                 @RequestParam String receivedGenre) {

        return this.seriesClient.getNextTwentySeriesByGenre(page, size, receivedGenre);
    }

    @GetMapping("/get-searched-series-count")
    public long getSearchedSeriesCount(@RequestParam String title) {
        return this.seriesClient.getSearchedSeriesCount(title);
    }

    @GetMapping("/get-series-by-title")
    public List<Series> getSeriesByTitle(@RequestParam String title) {
        List<Series> seriesByTitle = this.seriesClient.getSeriesByTitle(title);
        return seriesByTitle;
    }

    @GetMapping("/get-next-thirty-series")
    public List<CinemaRecordResponse> getEveryThirtySeries(@RequestParam(defaultValue = "0") int page,
                                                           @RequestParam(defaultValue = "10") int size) {

        return seriesClient.getEveryThirtySeries(page, size);
    }

    @GetMapping("/get-series-details")
    public Series getConcreteSeriesDetails(@RequestParam String id) {
        return this.seriesClient.getConcreteSeriesDetails(id);
    }

    @GetMapping("/get-all-series-count")
    public long getAllSeriesCount() {
        return this.seriesClient.getAllSeriesCount();
    }

    @PostMapping("/search-series")
    public void searchSeries(@RequestBody CinemaRecRequestDto cinemaRecRequestDto) throws IOException, InterruptedException {
        this.seriesClient.searchSeries(cinemaRecRequestDto.getRecordName());
    }

    @PutMapping("/react-series-comment")
    public ResponseEntity<Void> reactToSeriesComment(@RequestParam String commentId,
                                                      @RequestParam String reactionType,
                                                      Authentication authentication) {
        String clerkId = authentication.getName();

        try {
            UserResponse user = userClient.getCurrentUser(clerkId);

            seriesClient.reactToComment(
                    commentId,
                    reactionType,
                    user.getDisplayName(),
                    user.getDisplayName(),
                    user.getProfileImageURL(),
                    clerkId
            );
            return ResponseEntity.noContent().build();
        } catch (FeignException.NotFound e) {
            // lumo-user-svc doesn't have this clerkId yet, or the comment
            // itself is gone — either way there's nothing to react to.
            return ResponseEntity.status(HttpStatus.CONFLICT).build();
        } catch (FeignException e) {
            log.error("Failed to react to series comment for clerkId={}", clerkId, e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @GetMapping("/get-series-comments-reactions")
    public Map<String, CommentReactionsResponse> getSeriesCommentsReactions(@RequestParam String commentIds) {
        return this.seriesClient.getCommentsReactions(commentIds);
    }

    @GetMapping("/discover-series")
    public List<CinemaRecordResponse> discoverSeries(@RequestParam(required = false) String genre,
                                                      @RequestParam(required = false) String year,
                                                      @RequestParam(required = false) String actorName,
                                                      @RequestParam(required = false) String sort,
                                                      @RequestParam(defaultValue = "0") int page,
                                                      @RequestParam(defaultValue = "30") int size) {

        return this.seriesClient.discoverSeries(genre, year, actorName, sort, page, size);
    }

    @GetMapping("/discover-series-count")
    public long discoverSeriesCount(@RequestParam(required = false) String genre,
                                    @RequestParam(required = false) String year,
                                    @RequestParam(required = false) String actorName) {

        return this.seriesClient.discoverSeriesCount(genre, year, actorName);
    }

    @GetMapping("/suggest-series-actors")
    public List<Actor> suggestSeriesActors(@RequestParam String query) {
        return this.seriesClient.suggestSeriesActors(query);
    }
}
