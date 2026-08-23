package bg.stream_mates.backend.feather.lastviewed.dtos;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.Instant;

@Data
@Builder
@AllArgsConstructor
@NoArgsConstructor
public class LastViewedItemResponse {
    private String type;
    private String tmdbId;
    private String seriesId;
    private String posterURL;
    private String title;
    private String tmdbRating;
    private String videoURL;
    private String releaseDate;
    private Instant viewedAt;
}
