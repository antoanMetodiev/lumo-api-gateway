package bg.stream_mates.backend.commonData.controllers;

import bg.stream_mates.backend.commonData.dtos.ActorLatestWorks;
import bg.stream_mates.backend.feather.movies.client.MovieClient;
import bg.stream_mates.backend.feather.series.client.SeriesClient;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.List;

@RestController
@RequestMapping("/actor")
public class ActorController {

    private final MovieClient movieClient;
    private final SeriesClient seriesClient;

    @Autowired
    public ActorController(MovieClient movieClient, SeriesClient seriesClient) {
        this.movieClient = movieClient;
        this.seriesClient = seriesClient;
    }

    @GetMapping("/latest-works/{imdb_id}")
    public ResponseEntity<List<ActorLatestWorks>> getActorLatestWorks(@PathVariable(name = "imdb_id") String imdb_id) {
        List<ActorLatestWorks> actorLatestMovies = movieClient.getActorLatestMovies(imdb_id);
        List<ActorLatestWorks> actorLatestSeries = seriesClient.getActorLatestSeries(imdb_id);
        actorLatestMovies.addAll(actorLatestSeries);

        return ResponseEntity.ok().body(actorLatestMovies);
    }

}
