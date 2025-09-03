package bg.stream_mates.backend.feather.movies.controllers;

import bg.stream_mates.backend.feather.movies.client.MovieClient;
import bg.stream_mates.backend.feather.movies.models.entities.TrendingMovie;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.List;

@RestController
public class TrendingMoviesController {

    private final MovieClient movieClient;

    @Autowired
    public TrendingMoviesController(MovieClient movieClient) {
        this.movieClient = movieClient;
    }

    @GetMapping("/get-trending-movies")
    public List<TrendingMovie> getTrendingMovies() {
        return movieClient.getTrendingMovies();
    }
}
