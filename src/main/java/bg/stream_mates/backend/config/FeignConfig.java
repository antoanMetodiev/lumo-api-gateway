package bg.stream_mates.backend.config;

import com.fasterxml.jackson.databind.ObjectMapper;
import feign.RequestInterceptor;
import feign.codec.Decoder;
import feign.codec.Encoder;
import feign.jackson.JacksonDecoder;
import feign.jackson.JacksonEncoder;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class FeignConfig {

    private final ObjectMapper objectMapper;

    public FeignConfig(ObjectMapper objectMapper) {
        this.objectMapper = objectMapper;
    }

    @Bean
    public Decoder feignDecoder() {
        // Създава Jackson Decoder, който използва обекта ObjectMapper
        return new JacksonDecoder(objectMapper);  // JacksonDecoder е препоръчителният начин
    }

    @Bean
    public Encoder feignEncoder() {
        // Създава Jackson Encoder, който използва обекта ObjectMapper
        return new JacksonEncoder(objectMapper);  // JacksonEncoder е препоръчителният начин
    }

    /**
     * Stamps every outgoing Feign call (to lumo-user-svc, lumo-payments-svc,
     * etc.) with the shared internal secret, so those services can tell a
     * request really came from this gateway — see their own
     * GatewayAuthenticationFilter. Harmless for services that don't check it
     * (movies/series-svc just ignore the extra header).
     */
    @Bean
    public RequestInterceptor internalServiceAuthInterceptor(@Value("${internal.service-secret}") String internalServiceSecret) {
        return requestTemplate -> requestTemplate.header("X-Internal-Api-Key", internalServiceSecret);
    }
}
