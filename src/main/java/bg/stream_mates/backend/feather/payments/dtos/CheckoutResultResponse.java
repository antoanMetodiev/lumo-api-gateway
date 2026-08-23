package bg.stream_mates.backend.feather.payments.dtos;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

// Mirrors lumo-payments-svc's own CheckoutResultResponse 1:1 — Feign
// deserializes its JSON response straight into this class.
@Data
@Builder
@AllArgsConstructor
@NoArgsConstructor
public class CheckoutResultResponse {

    private Result result;

    public enum Result {
        PENDING, SUCCEEDED
    }
}
