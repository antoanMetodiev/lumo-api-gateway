package bg.stream_mates.backend.feather.watchlist.client;

import bg.stream_mates.backend.config.FeignConfig;
import bg.stream_mates.backend.feather.watchlist.dtos.WatchlistItemRequest;
import bg.stream_mates.backend.feather.watchlist.dtos.WatchlistItemResponse;
import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.*;

import java.util.List;

//@FeignClient(name = "lumo-user-svc-watchlist", url = "http://localhost:8083", configuration = FeignConfig.class)
@FeignClient(name = "lumo-user-svc-watchlist", url = "https://lumo-new-user-svc.onrender.com", configuration = FeignConfig.class)
public interface WatchlistClient {

    @GetMapping("/watchlist")
    List<WatchlistItemResponse> getWatchlist(@RequestHeader("X-Clerk-User-Id") String clerkId);

    @PostMapping(value = "/watchlist", consumes = "application/json")
    WatchlistItemResponse addToWatchlist(@RequestHeader("X-Clerk-User-Id") String clerkId, @RequestBody WatchlistItemRequest request);

    @DeleteMapping("/watchlist/{recordId}")
    void removeFromWatchlist(@RequestHeader("X-Clerk-User-Id") String clerkId, @PathVariable("recordId") String recordId);
}
