package bg.stream_mates.backend.utils;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import org.springframework.web.filter.OncePerRequestFilter;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.List;

/**
 * Authenticates INCOMING service-to-service calls into this gateway — the
 * reverse direction from ClerkAuthenticationFilter (which handles browser/FE
 * originated requests) and from FeignConfig's RequestInterceptor (which
 * stamps this same secret onto every OUTGOING call this gateway makes).
 *
 * Currently only lumo-payments-svc calls back in like this, to tell
 * lumo-user-svc (via this gateway) that a Stripe webhook changed someone's
 * plan — there's no live Clerk session for that call, so it can't go through
 * ClerkAuthenticationFilter at all. Same X-Internal-Api-Key secret already
 * shared with lumo-user-svc / lumo-payments-svc, just checked in the
 * opposite direction here.
 */
public class InternalServiceAuthenticationFilter extends OncePerRequestFilter {

    public static final String INTERNAL_API_KEY_HEADER = "X-Internal-Api-Key";

    private final byte[] internalApiKey;
    private final List<String> protectedPathPrefixes;

    public InternalServiceAuthenticationFilter(String internalApiKey, List<String> protectedPathPrefixes) {
        this.internalApiKey = internalApiKey.getBytes(StandardCharsets.UTF_8);
        this.protectedPathPrefixes = protectedPathPrefixes;
    }

    @Override
    protected void doFilterInternal(HttpServletRequest request, HttpServletResponse response, FilterChain filterChain)
            throws ServletException, IOException {

        String uri = request.getRequestURI();
        boolean isProtected = protectedPathPrefixes.stream()
                .anyMatch(prefix -> uri.equals(prefix) || uri.startsWith(prefix + "/"));

        if (!isProtected) {
            filterChain.doFilter(request, response);
            return;
        }

        String provided = request.getHeader(INTERNAL_API_KEY_HEADER);
        if (provided == null || !MessageDigest.isEqual(provided.getBytes(StandardCharsets.UTF_8), internalApiKey)) {
            response.setStatus(HttpServletResponse.SC_UNAUTHORIZED);
            return;
        }

        filterChain.doFilter(request, response);
    }
}
