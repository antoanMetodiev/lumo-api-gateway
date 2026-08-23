package bg.stream_mates.backend.feather.watchlist.dtos;

import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@AllArgsConstructor
@NoArgsConstructor
public class WatchlistItemRequest {

    @NotBlank
    private String recordId;

    private String tmdbId;
    private String title;
    private String posterImgURL;
    private String tmdbRating;
    private String type;
    private String videoId;
}
