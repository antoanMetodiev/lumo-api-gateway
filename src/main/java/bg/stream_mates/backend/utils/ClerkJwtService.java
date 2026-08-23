package bg.stream_mates.backend.utils;

import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.jwk.source.JWKSource;
import com.nimbusds.jose.jwk.source.JWKSourceBuilder;
import com.nimbusds.jose.proc.JWSKeySelector;
import com.nimbusds.jose.proc.JWSVerificationKeySelector;
import com.nimbusds.jose.proc.SecurityContext;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.proc.ConfigurableJWTProcessor;
import com.nimbusds.jwt.proc.DefaultJWTClaimsVerifier;
import com.nimbusds.jwt.proc.DefaultJWTProcessor;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.net.URL;
import java.util.HashSet;
import java.util.Set;

/**
 * Verifies Clerk session tokens (the JWT the frontend sends as
 * `Authorization: Bearer <token>`, from Clerk's `getToken()`) against Clerk's
 * own public JWKS — this gateway is now the ONLY service that does this;
 * lumo-user-svc and lumo-payments-svc trust a header this gateway forwards
 * instead (see their own GatewayAuthenticationFilter). RemoteJWKSet caches
 * the fetched keys and handles Clerk's periodic key rotation for us.
 */
@Component
public class ClerkJwtService {

    private final ConfigurableJWTProcessor<SecurityContext> jwtProcessor;

    public ClerkJwtService(
            @Value("${clerk.jwks-url}") String jwksUrl,
            @Value("${clerk.issuer}") String issuer
    ) throws java.net.MalformedURLException {
        JWKSource<SecurityContext> keySource = JWKSourceBuilder
                .create(new URL(jwksUrl))
                .retrying(true)
                .build();

        JWSKeySelector<SecurityContext> keySelector =
                new JWSVerificationKeySelector<>(JWSAlgorithm.RS256, keySource);

        DefaultJWTProcessor<SecurityContext> processor = new DefaultJWTProcessor<>();
        processor.setJWSKeySelector(keySelector);
        processor.setJWTClaimsSetVerifier(new DefaultJWTClaimsVerifier<>(
                new JWTClaimsSet.Builder().issuer(issuer).build(),
                new HashSet<>(Set.of("sub", "exp", "iat"))
        ));

        this.jwtProcessor = processor;
    }

    /**
     * Returns the Clerk user id (the `sub` claim) for a valid token, or null
     * if the token is missing, expired, or fails signature/issuer verification.
     */
    public String verifyAndGetUserId(String bearerToken) {
        if (bearerToken == null || bearerToken.isBlank()) return null;

        try {
            JWTClaimsSet claims = jwtProcessor.process(bearerToken, null);
            return claims.getSubject();
        } catch (Exception e) {
            return null;
        }
    }
}
