package bg.stream_mates.backend.feather.payments.dtos;

import jakarta.validation.constraints.Email;
import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@AllArgsConstructor
@NoArgsConstructor
public class CreateCheckoutSessionRequest {

    @NotBlank
    @Email
    private String email;
}
