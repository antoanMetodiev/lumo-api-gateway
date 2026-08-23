package bg.stream_mates.backend.feather.watchlist.controllers;

import bg.stream_mates.backend.feather.watchlist.client.WatchlistClient;
import bg.stream_mates.backend.feather.watchlist.dtos.WatchlistItemRequest;
import bg.stream_mates.backend.feather.watchlist.dtos.WatchlistItemResponse;
import feign.FeignException;
import jakarta.validation.Valid;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;

import java.util.List;

// Protected by ClerkAuthenticationFilter — see SecurityConfig.PROTECTED_PATHS
// ("/watchlist" also covers "/watchlist/{recordId}").
@Slf4j
@RestController
@RequestMapping("/watchlist")
public class WatchlistController {

    private final WatchlistClient watchlistClient;

    public WatchlistController(WatchlistClient watchlistClient) {
        this.watchlistClient = watchlistClient;
    }

    @GetMapping
    public ResponseEntity<List<WatchlistItemResponse>> getWatchlist(Authentication authentication) {
        try {
            return ResponseEntity.ok(watchlistClient.getWatchlist(authentication.getName()));
        } catch (FeignException e) {
            log.error("lumo-user-svc call failed for GET /watchlist", e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @PostMapping
    public ResponseEntity<WatchlistItemResponse> addToWatchlist(Authentication authentication, @RequestBody @Valid WatchlistItemRequest request) {
        try {
            return ResponseEntity.ok(watchlistClient.addToWatchlist(authentication.getName(), request));
        } catch (FeignException e) {
            log.error("lumo-user-svc call failed for POST /watchlist", e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @DeleteMapping("/{recordId}")
    public ResponseEntity<Void> removeFromWatchlist(Authentication authentication, @PathVariable String recordId) {
        try {
            watchlistClient.removeFromWatchlist(authentication.getName(), recordId);
            return ResponseEntity.noContent().build();
        } catch (FeignException e) {
            log.error("lumo-user-svc call failed for DELETE /watchlist/{}", recordId, e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }
}
