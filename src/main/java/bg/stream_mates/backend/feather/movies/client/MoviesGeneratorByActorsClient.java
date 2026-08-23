package bg.stream_mates.backend.feather.movies.client;

import bg.stream_mates.backend.config.FeignConfig;
import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;

//@FeignClient(name = "new-lumo-movies-generator-by-actors-svc", url = "http://localhost:9999", configuration = FeignConfig.class)
@FeignClient(name = "new-lumo-movies-generator-by-actors-svc", url = "https://new-lumo-movies-generator-by-actors.onrender.com", configuration = FeignConfig.class)
public interface MoviesGeneratorByActorsClient {

    @PostMapping("/search-movies")
    void searchMovies(@RequestBody String title);

    @PostMapping("/add-movies-by-actor/{imdb_id}")
    void addMoviesByActor(@PathVariable(name = "imdb_id") String imdb_id);

    @GetMapping("/heartbeat")
    void heartbeat();
}