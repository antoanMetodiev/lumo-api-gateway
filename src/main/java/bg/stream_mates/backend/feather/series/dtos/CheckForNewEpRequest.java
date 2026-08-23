package bg.stream_mates.backend.feather.series.dtos;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@AllArgsConstructor
@NoArgsConstructor
public class CheckForNewEpRequest {
    private String id;
    private String title;
}
