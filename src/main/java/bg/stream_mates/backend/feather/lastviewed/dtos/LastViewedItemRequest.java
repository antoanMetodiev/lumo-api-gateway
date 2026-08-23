package bg.stream_mates.backend.feather.lastviewed.dtos;

import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@AllArgsConstructor
@NoArgsConstructor
public class LastViewedItemRequest {

    @NotBlank
    private String type;

    private String tmdbId;
    private String seriesId;
    private String posterURL;

    @NotBlank
    private String title;

    private String tmdbRating;
    private String videoURL;
    private String releaseDate;
}
