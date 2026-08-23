package bg.stream_mates.backend.feather.lastviewed.controllers;

import bg.stream_mates.backend.feather.lastviewed.client.LastViewedClient;
import bg.stream_mates.backend.feather.lastviewed.dtos.LastViewedItemRequest;
import bg.stream_mates.backend.feather.lastviewed.dtos.LastViewedItemResponse;
import feign.FeignException;
import jakarta.validation.Valid;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;

import java.util.List;

// Protected by ClerkAuthenticationFilter — see SecurityConfig.PROTECTED_PATHS ("/last-viewed").
@Slf4j
@RestController
@RequestMapping("/last-viewed")
public class LastViewedController {

    private final LastViewedClient lastViewedClient;

    public LastViewedController(LastViewedClient lastViewedClient) {
        this.lastViewedClient = lastViewedClient;
    }

    @GetMapping
    public ResponseEntity<List<LastViewedItemResponse>> getLastViewed(Authentication authentication) {
        try {
            return ResponseEntity.ok(lastViewedClient.getLastViewed(authentication.getName()));
        } catch (FeignException e) {
            log.error("lumo-user-svc call failed for GET /last-viewed", e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @PostMapping
    public ResponseEntity<LastViewedItemResponse> recordView(Authentication authentication, @RequestBody @Valid LastViewedItemRequest request) {
        try {
            return ResponseEntity.ok(lastViewedClient.recordView(authentication.getName(), request));
        } catch (FeignException e) {
            log.error("lumo-user-svc call failed for POST /last-viewed", e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }
}
