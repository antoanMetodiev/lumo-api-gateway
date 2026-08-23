package bg.stream_mates.backend.feather.internal.dtos;

import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

// Deliberately a plain String, not a mirrored enum — this gateway doesn't
// need to understand the value, only pass it through to lumo-user-svc, which
// validates it against its own SubscriptionType enum.
@Data
@AllArgsConstructor
@NoArgsConstructor
public class UpdateSubscriptionRequest {

    @NotBlank
    private String subscriptionType;
}
