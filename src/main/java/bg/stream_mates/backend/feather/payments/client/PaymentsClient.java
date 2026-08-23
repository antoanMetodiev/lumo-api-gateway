package bg.stream_mates.backend.feather.payments.client;

import bg.stream_mates.backend.config.FeignConfig;
import bg.stream_mates.backend.feather.payments.dtos.CheckoutResultResponse;
import bg.stream_mates.backend.feather.payments.dtos.CheckoutSessionResponse;
import bg.stream_mates.backend.feather.payments.dtos.CreateCheckoutSessionRequest;
import bg.stream_mates.backend.feather.payments.dtos.PortalSessionResponse;
import bg.stream_mates.backend.feather.payments.dtos.SubscriptionResponse;
import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.*;

//@FeignClient(name = "lumo-payments-svc", url = "http://localhost:8084", configuration = FeignConfig.class)
@FeignClient(name = "lumo-payments-svc", url = "https://lumo-payments-svc.onrender.com", configuration = FeignConfig.class)
public interface PaymentsClient {

    @PostMapping(value = "/payments/checkout-session", consumes = "application/json")
    CheckoutSessionResponse createCheckoutSession(@RequestHeader("X-Clerk-User-Id") String clerkId, @RequestBody CreateCheckoutSessionRequest request);

    @PostMapping("/payments/portal-session")
    PortalSessionResponse createPortalSession(@RequestHeader("X-Clerk-User-Id") String clerkId);

    @GetMapping("/payments/subscription")
    SubscriptionResponse getSubscription(@RequestHeader("X-Clerk-User-Id") String clerkId);

    @GetMapping("/payments/checkout-result")
    CheckoutResultResponse getCheckoutResult(@RequestHeader("X-Clerk-User-Id") String clerkId, @RequestParam("sessionId") String sessionId);

    // No X-Clerk-User-Id here — service-to-service, same as UserClient's own
    // internal method (see InternalSubscriptionController). DELETE works fine
    // with Feign's default client, unlike PATCH.
    @DeleteMapping("/internal/subscriptions/{clerkId}")
    void deleteSubscription(@PathVariable("clerkId") String clerkId);

    @GetMapping("/heartbeat")
    void heartbeat();
}
