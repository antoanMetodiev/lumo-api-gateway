package bg.stream_mates.backend.feather.internal.controllers;

import bg.stream_mates.backend.feather.payments.client.PaymentsClient;
import feign.FeignException;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Service-to-service only — protected by InternalServiceAuthenticationFilter
 * (X-Internal-Api-Key), not ClerkAuthenticationFilter (see SecurityConfig).
 * The one caller is lumo-user-svc, right after it deletes its own copy of a
 * Clerk account, with no live Clerk session behind the call.
 */
@Slf4j
@RestController
@RequestMapping("/internal/subscriptions")
public class InternalSubscriptionController {

    private final PaymentsClient paymentsClient;

    public InternalSubscriptionController(PaymentsClient paymentsClient) {
        this.paymentsClient = paymentsClient;
    }

    @DeleteMapping("/{clerkId}")
    public ResponseEntity<Void> deleteSubscription(@PathVariable String clerkId) {
        try {
            paymentsClient.deleteSubscription(clerkId);
            return ResponseEntity.noContent().build();
        } catch (FeignException e) {
            log.error("lumo-payments-svc call failed for DELETE /internal/subscriptions/{}", clerkId, e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }
}
