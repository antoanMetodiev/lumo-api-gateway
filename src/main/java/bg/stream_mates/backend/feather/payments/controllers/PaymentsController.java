package bg.stream_mates.backend.feather.payments.controllers;

import bg.stream_mates.backend.feather.payments.client.PaymentsClient;
import bg.stream_mates.backend.feather.payments.dtos.CheckoutResultResponse;
import bg.stream_mates.backend.feather.payments.dtos.CheckoutSessionResponse;
import bg.stream_mates.backend.feather.payments.dtos.CreateCheckoutSessionRequest;
import bg.stream_mates.backend.feather.payments.dtos.PortalSessionResponse;
import bg.stream_mates.backend.feather.payments.dtos.SubscriptionResponse;
import feign.FeignException;
import jakarta.validation.Valid;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.*;

// Protected by ClerkAuthenticationFilter — see SecurityConfig.PROTECTED_PATHS ("/payments").
@Slf4j
@RestController
@RequestMapping("/payments")
public class PaymentsController {

    private final PaymentsClient paymentsClient;

    public PaymentsController(PaymentsClient paymentsClient) {
        this.paymentsClient = paymentsClient;
    }

    @PostMapping("/checkout-session")
    public ResponseEntity<CheckoutSessionResponse> createCheckoutSession(Authentication authentication, @RequestBody @Valid CreateCheckoutSessionRequest request) {
        try {
            return ResponseEntity.ok(paymentsClient.createCheckoutSession(authentication.getName(), request));
        } catch (FeignException e) {
            log.error("lumo-payments-svc call failed for POST /payments/checkout-session", e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @PostMapping("/portal-session")
    public ResponseEntity<PortalSessionResponse> createPortalSession(Authentication authentication) {
        try {
            return ResponseEntity.ok(paymentsClient.createPortalSession(authentication.getName()));
        } catch (FeignException.NotFound e) {
            return ResponseEntity.notFound().build();
        } catch (FeignException e) {
            log.error("lumo-payments-svc call failed for POST /payments/portal-session", e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @GetMapping("/subscription")
    public ResponseEntity<SubscriptionResponse> getSubscription(Authentication authentication) {
        try {
            return ResponseEntity.ok(paymentsClient.getSubscription(authentication.getName()));
        } catch (FeignException e) {
            log.error("lumo-payments-svc call failed for GET /payments/subscription", e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @GetMapping("/checkout-result")
    public ResponseEntity<CheckoutResultResponse> getCheckoutResult(Authentication authentication, @RequestParam String sessionId) {
        try {
            return ResponseEntity.ok(paymentsClient.getCheckoutResult(authentication.getName(), sessionId));
        } catch (FeignException.NotFound e) {
            return ResponseEntity.notFound().build();
        } catch (FeignException e) {
            log.error("lumo-payments-svc call failed for GET /payments/checkout-result", e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }
}
