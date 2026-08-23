package bg.stream_mates.backend.feather.movies.models.dtos;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

// Shared by both movies-svc and series-svc comment reactions responses.
@Data
@AllArgsConstructor
@NoArgsConstructor
public class ReactorDto {

    private String userId;
    private String username;
    private String fullName;
    private String imgURL;
    private String createdAt;
}
