package bg.stream_mates.backend.heartbeat;

import bg.stream_mates.backend.feather.movies.client.MovieClient;
import bg.stream_mates.backend.feather.series.client.SeriesClient;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class Heartbeat {

    private final MovieClient movieClient;
    private final SeriesClient seriesClient;

    @Autowired
    public Heartbeat(MovieClient movieClient, SeriesClient seriesClient) {
        this.movieClient = movieClient;
        this.seriesClient = seriesClient;
    }

    @GetMapping("/heartbeat")
    public ResponseEntity<Void> heartbeat() {
        try {
            movieClient.getAllMoviesCount();
            seriesClient.getAllSeriesCount();
        } catch (Exception e) {
            System.out.println("Heartbeat failed: " + e.getMessage());
            return new ResponseEntity<>(HttpStatus.INTERNAL_SERVER_ERROR);
        }

        return ResponseEntity.ok().build();
    }
}
