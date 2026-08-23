package bg.stream_mates.backend.feather.payments.dtos;

// Mirrors lumo-payments-svc's own SubscriptionStatus 1:1 — Feign deserializes
// its JSON response straight into this enum, so a value missing here breaks
// GET /payments/subscription with a Jackson error instead of a normal
// response.
public enum SubscriptionStatus {
    NONE,
    INCOMPLETE,
    INCOMPLETE_EXPIRED,
    TRIALING,
    ACTIVE,
    PAST_DUE,
    CANCELED,
    UNPAID,
    PAUSED
}
