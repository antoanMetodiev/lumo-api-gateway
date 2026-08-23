package bg.stream_mates.backend.feather.movies.models.dtos;

import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

// Shared by both MovieCommentsController and SeriesController. Deliberately
// just these two fields — author identity (username/full name/avatar/id) and
// createdAt used to come from the client body, which meant anyone could post
// a comment as anyone else. Both controllers now derive that from the
// verified Clerk session and lumo-user-svc's own record instead.
@Builder
@Data
@NoArgsConstructor
@AllArgsConstructor
public class PostCommentRequest {

    @NotBlank(message = "Comment text cannot be blank!")
    private String commentText;

    @NotBlank(message = "Cinema Record id cannot be blank!")
    private String cinemaRecordId;
}
