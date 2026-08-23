package bg.stream_mates.backend.feather.users.client;

import bg.stream_mates.backend.config.FeignConfig;
import bg.stream_mates.backend.feather.internal.dtos.UpdateSubscriptionRequest;
import bg.stream_mates.backend.feather.users.dtos.UserResponse;
import org.springframework.cloud.openfeign.FeignClient;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;

//@FeignClient(name = "lumo-user-svc", url = "http://localhost:8083", configuration = FeignConfig.class)
@FeignClient(name = "lumo-user-svc", url = "https://lumo-new-user-svc.onrender.com", configuration = FeignConfig.class)
public interface UserClient {

    // The internal secret is attached automatically to every call by
    // FeignConfig's RequestInterceptor — only the already-verified Clerk user
    // id needs to be passed explicitly here.
    @GetMapping("/user/me")
    UserResponse getCurrentUser(@RequestHeader("X-Clerk-User-Id") String clerkId);

    // No X-Clerk-User-Id here — this is a service-to-service call, the
    // internal secret alone is the auth (see InternalServiceAuthenticationFilter
    // / lumo-user-svc's GatewayAuthenticationFilter internal path list).
    // POST, not PATCH — Feign's default client (java.net.HttpURLConnection-based)
    // can't send PATCH at all ("Invalid HTTP method: PATCH").
    @PostMapping(value = "/internal/users/{clerkId}/subscription", consumes = "application/json")
    void updateSubscription(@PathVariable("clerkId") String clerkId, @RequestBody UpdateSubscriptionRequest request);

    @GetMapping("/heartbeat")
    void heartbeat();
}
