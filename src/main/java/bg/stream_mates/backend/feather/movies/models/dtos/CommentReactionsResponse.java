package bg.stream_mates.backend.feather.movies.models.dtos;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.ArrayList;
import java.util.List;

// Shared by both movies-svc and series-svc comment reactions responses.
@Data
@AllArgsConstructor
@NoArgsConstructor
public class CommentReactionsResponse {

    private List<ReactorDto> likes = new ArrayList<>();
    private List<ReactorDto> dislikes = new ArrayList<>();
}
