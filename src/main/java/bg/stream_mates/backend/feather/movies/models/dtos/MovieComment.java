package bg.stream_mates.backend.feather.movies.models.dtos;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import lombok.experimental.Accessors;

import java.util.UUID;

@Data
@AllArgsConstructor
@NoArgsConstructor
@Accessors(chain = true)
public class MovieComment {

    private UUID id;

    private String commentText;

    private String authorUsername;

    private String authorFullName;

    private String authorId;

    private String authorImgURL;

    // No longer collected — nullable since movies-svc's own column is now
    // nullable too (see that service's MovieComment entity).
    private Double rating;

    private String createdAt;
}
