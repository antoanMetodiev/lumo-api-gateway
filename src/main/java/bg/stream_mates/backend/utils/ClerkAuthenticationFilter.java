package bg.stream_mates.backend.utils;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import org.springframework.security.authentication.UsernamePasswordAuthenticationToken;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.filter.OncePerRequestFilter;

import java.io.IOException;
import java.util.List;

/**
 * Verifies the Clerk session token on protected endpoints (/user/**,
 * /watchlist/**, /last-viewed/**, /payments/**) and, if valid, sets the
 * Clerk user id as the request's principal so controllers can read it via
 * SecurityContextHolder / an @AuthenticationPrincipal — same mechanism
 * lumo-user-svc used to run itself before this gateway became the single
 * place that talks to Clerk's JWKS. Public endpoints (catalog browsing,
 * webhooks, heartbeat) skip this entirely — see SecurityConfig.
 */
public class ClerkAuthenticationFilter extends OncePerRequestFilter {

    private final ClerkJwtService clerkJwtService;
    private final List<String> protectedPathPrefixes;

    public ClerkAuthenticationFilter(ClerkJwtService clerkJwtService, List<String> protectedPathPrefixes) {
        this.clerkJwtService = clerkJwtService;
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

        String authHeader = request.getHeader("Authorization");
        String token = (authHeader != null && authHeader.startsWith("Bearer "))
                ? authHeader.substring("Bearer ".length())
                : null;

        String clerkUserId = clerkJwtService.verifyAndGetUserId(token);
        if (clerkUserId == null) {
            response.setStatus(HttpServletResponse.SC_UNAUTHORIZED);
            return;
        }

        SecurityContextHolder.getContext().setAuthentication(
                new UsernamePasswordAuthenticationToken(clerkUserId, null, List.of())
        );

        filterChain.doFilter(request, response);
    }
}
