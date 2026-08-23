package bg.stream_mates.backend.feather.movies.controllers;

import bg.stream_mates.backend.commonData.dtos.Actor;
import bg.stream_mates.backend.commonData.dtos.CinemaRecRequestDto;
import bg.stream_mates.backend.commonData.dtos.CinemaRecordResponse;
import bg.stream_mates.backend.feather.movies.client.MovieClient;
import bg.stream_mates.backend.feather.movies.client.MoviesGeneratorByActorsClient;
import bg.stream_mates.backend.feather.movies.client.MoviesGeneratorClient;
import bg.stream_mates.backend.feather.movies.models.entities.Movie;
import jakarta.validation.Valid;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.io.IOException;
import java.util.ArrayList;
import java.util.List;

@RestController
public class MovieController {

    private final MovieClient movieClient;
    private final MoviesGeneratorClient moviesGeneratorClient;
    private final MoviesGeneratorByActorsClient moviesGeneratorByActorsClient;

    @Autowired
    public MovieController(MovieClient movieClient, MoviesGeneratorClient moviesGeneratorClient, MoviesGeneratorByActorsClient moviesGeneratorByActorsClient) {
        this.movieClient = movieClient;
        this.moviesGeneratorClient = moviesGeneratorClient;
        this.moviesGeneratorByActorsClient = moviesGeneratorByActorsClient;
    }

    @PostMapping("/add-movies-by-actor/{imdb_id}")
    public ResponseEntity<Void> addMoviesByActor(@PathVariable(name = "imdb_id") String imdb_id) {

        try {
            moviesGeneratorByActorsClient.addMoviesByActor(imdb_id);
            return ResponseEntity.ok().build();
        } catch (Exception e) {
            return ResponseEntity.internalServerError().build();
        }
    }

    @PostMapping("/search-movies")
    public void searchMovies(@RequestBody @Valid CinemaRecRequestDto cinemaRecRequestDto) throws IOException, InterruptedException {
        moviesGeneratorClient.searchMovies(cinemaRecRequestDto.getRecordName());
    }

    @GetMapping("/search-movies-matching-results/{searchTitle}")
    public ResponseEntity<List<CinemaRecordResponse>> searchMoviesMatchingResults(@PathVariable(value = "searchTitle") String searchTitle) {

        System.out.println();
        List<CinemaRecordResponse> response = movieClient.searchMoviesMatchingResults(searchTitle);
        return ResponseEntity.ok(response);
    }

    @GetMapping("/get-searched-movies-count")
    public long getSearchedMoviesCount(@RequestParam String title) {
        return this.movieClient.getSearchedMoviesCount(title);
    }

    @GetMapping("/get-movies-count-by-genre")
    public long findMoviesCountByGenre(@RequestParam String receivedGenre) {
        String genres = receivedGenre;
        return this.movieClient.findMoviesCountByGenre(genres);
    }

    @GetMapping("/get-next-twenty-movies-by-genre")
    public List<CinemaRecordResponse> getNextTwentyMoviesByGenre(@RequestParam(defaultValue = "0") int page,
                                                                 @RequestParam(defaultValue = "20") int size,
                                                                 @RequestParam String receivedGenre) {

        return this.movieClient.getNextTwentyMoviesByGenre(page, size, receivedGenre);  // Предаваме жанра и Pageable на сървиса
    }

    @GetMapping("/get-next-thirty-movies")
    public List<CinemaRecordResponse> getEveryThirtyMovies(@RequestParam(defaultValue = "0") int page,
                                                           @RequestParam(defaultValue = "10") int size) {

        List<CinemaRecordResponse> everyThirtyMovies = this.movieClient.getEveryThirtyMovies(page, size);
        return everyThirtyMovies;
    }

    @GetMapping("/get-movies-by-title")
    public List<Movie> getMoviesByTitle(@RequestParam String title) {
        if (title == null || title.trim().isEmpty()) return new ArrayList<Movie>();

        List<Movie> moviesByTitle = this.movieClient.getMoviesByTitle(title);
        return moviesByTitle;
    }

    @GetMapping("/get-movie-details")
    public Movie getConcreteMovieDetails(@RequestParam String movieId) {
        if (movieId == null || movieId.trim().isEmpty()) return new Movie();
        Movie movie = this.movieClient.getConcreteMovieDetails(movieId);
        return movie;
    }

    @GetMapping("/get-all-movies-count")
    public long getAllMoviesCount() {
        return this.movieClient.getAllMoviesCount();
    }

    @GetMapping("/discover-movies")
    public List<CinemaRecordResponse> discoverMovies(@RequestParam(required = false) String genre,
                                                      @RequestParam(required = false) String year,
                                                      @RequestParam(required = false) String actorName,
                                                      @RequestParam(required = false) String sort,
                                                      @RequestParam(defaultValue = "0") int page,
                                                      @RequestParam(defaultValue = "30") int size) {

        return this.movieClient.discoverMovies(genre, year, actorName, sort, page, size);
    }

    @GetMapping("/discover-movies-count")
    public long discoverMoviesCount(@RequestParam(required = false) String genre,
                                    @RequestParam(required = false) String year,
                                    @RequestParam(required = false) String actorName) {

        return this.movieClient.discoverMoviesCount(genre, year, actorName);
    }

    @GetMapping("/suggest-movie-actors")
    public List<Actor> suggestMovieActors(@RequestParam String query) {
        return this.movieClient.suggestMovieActors(query);
    }
}