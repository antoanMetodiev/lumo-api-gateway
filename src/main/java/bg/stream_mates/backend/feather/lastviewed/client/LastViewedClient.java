package bg.stream_mates.backend.feather.lastviewed.client;

import bg.stream_mates.backend.config.FeignConfig;
import bg.stream_mates.backend.feather.lastviewed.dtos.LastViewedItemRequest;
import bg.stream_mates.backend.feather.lastviewed.dtos.LastViewedItemResponse;
import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;

import java.util.List;

//@FeignClient(name = "lumo-user-svc-last-viewed", url = "http://localhost:8083", configuration = FeignConfig.class)
@FeignClient(name = "lumo-user-svc-last-viewed", url = "https://lumo-new-user-svc.onrender.com", configuration = FeignConfig.class)
public interface LastViewedClient {

    @GetMapping("/last-viewed")
    List<LastViewedItemResponse> getLastViewed(@RequestHeader("X-Clerk-User-Id") String clerkId);

    @PostMapping(value = "/last-viewed", consumes = "application/json")
    LastViewedItemResponse recordView(@RequestHeader("X-Clerk-User-Id") String clerkId, @RequestBody LastViewedItemRequest request);
}
