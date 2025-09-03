package bg.stream_mates.backend.feather.movies.models.entities;

import bg.stream_mates.backend.commonData.CinemaRecord;
import jakarta.persistence.*;
import lombok.*;

import java.util.UUID;

@Entity
@Table(name = "trending_movies")
@Getter
@Setter
@Builder
@AllArgsConstructor
@NoArgsConstructor
public class TrendingMovie extends CinemaRecord {

    @Id
    @GeneratedValue(strategy = GenerationType.UUID)
    private UUID id;

    @Version
    private Long version;

    @Column(name = "movie_id", nullable = false, unique = true)
    private String movieId;

    @Column(name = "trailer_video_url", nullable = false)
    private String trailerVideoURL;

    @Column(name = "logo_url")
    private String logoURL;
}
