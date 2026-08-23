package bg.stream_mates.backend.feather.movies.models.dtos;

import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

// Shared by both MovieCommentsController and SeriesController.
@Builder
@Data
@NoArgsConstructor
@AllArgsConstructor
public class UpdateCommentRequest {

    @NotBlank(message = "Comment text cannot be blank!")
    private String commentText;
}
