package bg.stream_mates.backend.feather.payments.dtos;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.Instant;

@Data
@Builder
@AllArgsConstructor
@NoArgsConstructor
public class SubscriptionResponse {
    private PlanType plan;
    private SubscriptionStatus status;
    private Instant currentPeriodEnd;
    private boolean cancelAtPeriodEnd;
}
