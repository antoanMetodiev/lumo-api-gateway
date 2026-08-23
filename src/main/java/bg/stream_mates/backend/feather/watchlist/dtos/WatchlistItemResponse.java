package bg.stream_mates.backend.feather.watchlist.dtos;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.Instant;

@Data
@Builder
@AllArgsConstructor
@NoArgsConstructor
public class WatchlistItemResponse {
    private String recordId;
    private String tmdbId;
    private String title;
    private String posterImgURL;
    private String tmdbRating;
    private String type;
    private String videoId;
    private Instant addedAt;
}
