package bg.stream_mates.backend.commonData.dtos;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@AllArgsConstructor
@NoArgsConstructor
@Builder
public class ActorLatestWorks {
    private String seriesId;
    private String tmdbId;
    private String videoURL;
    private String posterURL;
    private String title;
    private String tmdbRating;
    private String releaseDate;
    private String TYPE;
}
