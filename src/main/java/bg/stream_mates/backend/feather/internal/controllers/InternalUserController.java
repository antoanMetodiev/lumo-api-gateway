package bg.stream_mates.backend.feather.internal.controllers;

import bg.stream_mates.backend.feather.internal.dtos.UpdateSubscriptionRequest;
import bg.stream_mates.backend.feather.users.client.UserClient;
import bg.stream_mates.backend.feather.users.dtos.UserResponse;
import feign.FeignException;
import jakarta.validation.Valid;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Service-to-service only — protected by InternalServiceAuthenticationFilter
 * (X-Internal-Api-Key), not ClerkAuthenticationFilter (see SecurityConfig).
 * Callers: lumo-payments-svc, syncing a Clerk user's plan after a Stripe
 * webhook (POST), or checking whether that sync has actually landed yet for
 * its post-checkout polling endpoint (GET) — see CheckoutService.getCheckoutResult.
 * No live Clerk session behind either call.
 */
@Slf4j
@RestController
@RequestMapping("/internal/users")
public class InternalUserController {

    private final UserClient userClient;

    public InternalUserController(UserClient userClient) {
        this.userClient = userClient;
    }

    // POST, not PATCH: Feign's default HTTP client wraps java.net.HttpURLConnection,
    // which hard-rejects PATCH ("Invalid HTTP method: PATCH") — a long-standing JDK
    // quirk, not something worth pulling in an alternate Feign HTTP client to work
    // around for one internal endpoint.
    @PostMapping("/{clerkId}/subscription")
    public ResponseEntity<Void> updateSubscription(@PathVariable String clerkId, @RequestBody @Valid UpdateSubscriptionRequest request) {
        try {
            userClient.updateSubscription(clerkId, request);
            return ResponseEntity.noContent().build();
        } catch (FeignException.NotFound e) {
            return ResponseEntity.notFound().build();
        } catch (FeignException e) {
            log.error("lumo-user-svc call failed for POST /internal/users/{}/subscription", clerkId, e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @GetMapping("/{clerkId}")
    public ResponseEntity<UserResponse> getUser(@PathVariable String clerkId) {
        try {
            return ResponseEntity.ok(userClient.getCurrentUser(clerkId));
        } catch (FeignException.NotFound e) {
            return ResponseEntity.notFound().build();
        } catch (FeignException e) {
            log.error("lumo-user-svc call failed for GET /internal/users/{}", clerkId, e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }
}
