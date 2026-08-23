package bg.stream_mates.backend.feather.users.controllers;

import bg.stream_mates.backend.feather.users.client.UserClient;
import bg.stream_mates.backend.feather.users.dtos.UserResponse;
import feign.FeignException;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.Authentication;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

// Protected by ClerkAuthenticationFilter — see SecurityConfig.PROTECTED_PATHS ("/user").
@Slf4j
@RestController
@RequestMapping("/user")
public class UserController {

    private final UserClient userClient;

    public UserController(UserClient userClient) {
        this.userClient = userClient;
    }

    /**
     * authentication.getName() is the verified Clerk user id, set by
     * ClerkAuthenticationFilter. 404 here means lumo-user-svc's Clerk webhook
     * hasn't synced this account yet — near-instant after sign-up, but a real
     * race the frontend should tolerate rather than treat as a hard failure.
     */
    @GetMapping("/me")
    public ResponseEntity<UserResponse> getCurrentUser(Authentication authentication) {
        try {
            return ResponseEntity.ok(userClient.getCurrentUser(authentication.getName()));
        } catch (FeignException.NotFound e) {
            return ResponseEntity.notFound().build();
        } catch (FeignException e) {
            log.error("lumo-user-svc call failed for GET /user/me", e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }
}
