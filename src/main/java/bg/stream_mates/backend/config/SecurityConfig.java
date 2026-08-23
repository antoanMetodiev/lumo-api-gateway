package bg.stream_mates.backend.config;

import bg.stream_mates.backend.utils.ClerkAuthenticationFilter;
import bg.stream_mates.backend.utils.ClerkJwtService;
import bg.stream_mates.backend.utils.InternalServiceAuthenticationFilter;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.annotation.web.configurers.AbstractHttpConfigurer;
import org.springframework.security.config.http.SessionCreationPolicy;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.authentication.www.BasicAuthenticationFilter;
import org.springframework.web.cors.CorsConfiguration;
import org.springframework.web.cors.CorsConfigurationSource;
import org.springframework.web.cors.UrlBasedCorsConfigurationSource;

import java.util.Arrays;
import java.util.List;

@Configuration
public class SecurityConfig {

    // Path prefixes that require a valid Clerk session token
    // (Authorization: Bearer <token>) — everything that used to be verified
    // directly by lumo-user-svc / lumo-payments-svc now gets verified once,
    // here. Catalog browsing (movies/series/actor) stays open, same as before.
    //
    // The comment-mutation routes are exact matches, not real prefixes —
    // movies-svc/series-svc predate this gateway's routing conventions and
    // expose top-level paths with no shared "/comments" parent to protect as
    // a group. Reading comments (get-next-10-*-comments) stays open.
    private static final List<String> PROTECTED_PATHS = List.of(
            "/user", "/watchlist", "/last-viewed", "/payments",
            "/post-movie-comment", "/update-movie-comment", "/delete-movie-comment", "/react-movie-comment",
            "/post-series-comment", "/update-series-comment", "/delete-series-comment", "/react-series-comment"
    );

    // Service-to-service paths — calls from lumo-payments-svc that have no
    // live Clerk session behind them (see InternalServiceAuthenticationFilter).
    private static final List<String> INTERNAL_PATHS = List.of("/internal");

    @Bean
    public ClerkAuthenticationFilter clerkAuthenticationFilter(ClerkJwtService clerkJwtService) {
        return new ClerkAuthenticationFilter(clerkJwtService, PROTECTED_PATHS);
    }

    @Bean
    public InternalServiceAuthenticationFilter internalServiceAuthenticationFilter(@Value("${internal.service-secret}") String internalServiceSecret) {
        return new InternalServiceAuthenticationFilter(internalServiceSecret, INTERNAL_PATHS);
    }

    @Bean
    public SecurityFilterChain securityFilterChain(
            HttpSecurity http,
            ClerkAuthenticationFilter clerkAuthenticationFilter,
            InternalServiceAuthenticationFilter internalServiceAuthenticationFilter
    ) throws Exception {
        http
                .cors(cors -> cors.configurationSource(corsConfigurationSource()))
                .csrf(AbstractHttpConfigurer::disable)
                .sessionManagement(session -> session.sessionCreationPolicy(SessionCreationPolicy.STATELESS))
                .authorizeHttpRequests(auth -> auth.anyRequest().permitAll())
                .addFilterBefore(clerkAuthenticationFilter, BasicAuthenticationFilter.class)
                .addFilterBefore(internalServiceAuthenticationFilter, BasicAuthenticationFilter.class);

        return http.build();
    }

    @Bean
    public CorsConfigurationSource corsConfigurationSource() {
        CorsConfiguration configuration = new CorsConfiguration();

        // The old Vite frontend + its Netlify deploy. The Next.js app never
        // calls this gateway from the browser — its server calls it directly
        // (no Origin header involved), and its few genuinely client-side
        // catalog calls go through its own same-origin proxy instead. Add the
        // Next.js app's own origin here only if that ever changes.
        configuration.setAllowedOriginPatterns(Arrays.asList(
                "https://lumoflix.vercel.app",
                "http://localhost:5173"
        ));
        configuration.setAllowedMethods(Arrays.asList("GET", "POST", "PUT", "DELETE", "OPTIONS"));
        configuration.setAllowCredentials(true);
        configuration.setAllowedHeaders(Arrays.asList(
                "Authorization", "Content-Type", "X-Custom-Logout", "X-Requested-With"
        ));
        configuration.setExposedHeaders(Arrays.asList("Set-Cookie", "X-Custom-Logout"));

        UrlBasedCorsConfigurationSource source = new UrlBasedCorsConfigurationSource();
        source.registerCorsConfiguration("/**", configuration);
        return source;
    }
}
