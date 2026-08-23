package bg.stream_mates.backend.feather.users.dtos;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.time.Instant;

/**
 * Mirrors lumo-user-svc's User entity — a separate copy rather than a shared
 * dependency, same as every other cross-service shape in this gateway
 * (CinemaRecordResponse, Movie, Series, ...).
 */
@Data
@Builder
@AllArgsConstructor
@NoArgsConstructor
public class UserResponse {
    private String clerkId;
    private String email;
    private String displayName;
    private String profileImageURL;
    private String subscriptionType;
    private Instant createdAt;
    private Instant updatedAt;
}
