package bg.stream_mates.backend.feather.movies.controllers;

import bg.stream_mates.backend.feather.movies.client.MovieClient;
import bg.stream_mates.backend.feather.movies.models.dtos.CommentReactionsResponse;
import bg.stream_mates.backend.feather.movies.models.dtos.MovieComment;
import bg.stream_mates.backend.feather.movies.models.dtos.PostCommentRequest;
import bg.stream_mates.backend.feather.movies.models.dtos.UpdateCommentRequest;
import bg.stream_mates.backend.feather.users.client.UserClient;
import bg.stream_mates.backend.feather.users.dtos.UserResponse;
import feign.FeignException;
import jakarta.validation.Valid;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;

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
public class MovieCommentsController {

    private final MovieClient movieClient;
    private final UserClient userClient;

    public MovieCommentsController(MovieClient movieClient, UserClient userClient) {
        this.movieClient = movieClient;
        this.userClient = userClient;
    }

    @DeleteMapping("/delete-movie-comment")
    public ResponseEntity<Void> deleteMovieComment(@RequestParam String commentId,
                                                    @RequestParam String movieId,
                                                    Authentication authentication) {
        try {
            movieClient.deleteMovieComment(commentId, movieId, authentication.getName());
            return ResponseEntity.noContent().build();
        } catch (FeignException.Forbidden e) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).build();
        } catch (FeignException.NotFound e) {
            return ResponseEntity.notFound().build();
        } catch (FeignException e) {
            log.error("lumo-movies-svc call failed for DELETE /delete-movie-comment", e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @PutMapping("/update-movie-comment")
    public ResponseEntity<Void> updateMovieComment(@RequestParam String commentId,
                                                    @RequestParam String movieId,
                                                    @RequestBody @Valid UpdateCommentRequest request,
                                                    Authentication authentication) {
        try {
            movieClient.updateMovieComment(commentId, movieId, request.getCommentText(), authentication.getName());
            return ResponseEntity.noContent().build();
        } catch (FeignException.Forbidden e) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).build();
        } catch (FeignException.NotFound e) {
            return ResponseEntity.notFound().build();
        } catch (FeignException e) {
            log.error("lumo-movies-svc call failed for PUT /update-movie-comment", e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @GetMapping("/get-next-10-movie-comments")
    public List<MovieComment> getNext10Comments(@RequestParam int order,
                                                @RequestParam String currentCinemaRecordId) {
        return this.movieClient.getNext10Comments(order, currentCinemaRecordId);
    }

    @PostMapping("/post-movie-comment")
    public ResponseEntity<Void> postComment(@RequestBody @Valid PostCommentRequest postCommentRequest, Authentication authentication) {
        String clerkId = authentication.getName();

        try {
            UserResponse user = userClient.getCurrentUser(clerkId);

            movieClient.postComment(
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
            log.error("Failed to post movie comment for clerkId={}", clerkId, e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @PutMapping("/react-movie-comment")
    public ResponseEntity<Void> reactToMovieComment(@RequestParam String commentId,
                                                    @RequestParam String reactionType,
                                                    Authentication authentication) {
        String clerkId = authentication.getName();

        try {
            UserResponse user = userClient.getCurrentUser(clerkId);

            movieClient.reactToComment(
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
            log.error("Failed to react to movie comment for clerkId={}", clerkId, e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @GetMapping("/get-movie-comments-reactions")
    public Map<String, CommentReactionsResponse> getMovieCommentsReactions(@RequestParam String commentIds) {
        return this.movieClient.getCommentsReactions(commentIds);
    }
}
