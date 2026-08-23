package bg.stream_mates.backend.feather.series.models;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import lombok.experimental.Accessors;

import java.util.UUID;

@Data
@AllArgsConstructor
@NoArgsConstructor
@Accessors(chain = true)
public class SeriesComment {

    // series-svc's own entity always has this; was simply never mirrored
    // here before, which would have made it impossible for a client to know
    // which comment to edit/delete.
    private UUID id;

    private String commentText;

    private String authorUsername;

    private String authorFullName;

    private String authorImgURL;

    // Clerk user ids (e.g. "user_2abc...") aren't UUIDs.
    private String authorId;

    // No longer collected — nullable since series-svc's own column is now
    // nullable too (see that service's SeriesComment entity).
    private Double rating;

    private String createdAt;

    private Series series;
}
