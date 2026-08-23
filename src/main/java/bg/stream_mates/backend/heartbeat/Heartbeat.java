package bg.stream_mates.backend.heartbeat;

import bg.stream_mates.backend.feather.movies.client.MovieClient;
import bg.stream_mates.backend.feather.payments.client.PaymentsClient;
import bg.stream_mates.backend.feather.series.client.SeriesClient;
import bg.stream_mates.backend.feather.users.client.UserClient;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
public class Heartbeat {

    private final MovieClient movieClient;
    private final SeriesClient seriesClient;
    private final UserClient userClient;
    private final PaymentsClient paymentsClient;

    @Autowired
    public Heartbeat(MovieClient movieClient, SeriesClient seriesClient, UserClient userClient, PaymentsClient paymentsClient) {
        this.movieClient = movieClient;
        this.seriesClient = seriesClient;
        this.userClient = userClient;
        this.paymentsClient = paymentsClient;
    }

    // Fans out this gateway's own self-ping (see SelfHeartBeatScheduler, every
    // 4 minutes) to every downstream service, so lumo-user-svc and
    // lumo-payments-svc stay warm too — Render's free tier puts an idle
    // service to sleep after ~15 minutes, and a cold Spring Boot boot can
    // easily take 30-60+ seconds, which used to blow straight through
    // CheckoutService's post-checkout polling timeout and show a real
    // successful purchase as "failed" on the frontend.
    @GetMapping("/heartbeat")
    public ResponseEntity<Void> heartbeat() {
        try {
            Thread.ofVirtual().start(movieClient::heartbeat);
            Thread.ofVirtual().start(seriesClient::heartbeat);
            Thread.ofVirtual().start(userClient::heartbeat);
            Thread.ofVirtual().start(paymentsClient::heartbeat);
        } catch (Exception e) {
            System.out.println("Heartbeat failed: " + e.getMessage());
            return new ResponseEntity<>(HttpStatus.INTERNAL_SERVER_ERROR);
        }

        return ResponseEntity.ok().build();
    }
}
