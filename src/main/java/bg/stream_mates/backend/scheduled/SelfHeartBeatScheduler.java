package bg.stream_mates.backend.scheduled;

import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestTemplate;

@Component
public class SelfHeartBeatScheduler {

    private final RestTemplate restTemplate = new RestTemplate();

    @Scheduled(fixedRate = 240_000) // на всеки 240 000 ms = 4 минути
    public void heartBeatSelf() {
        try {
            System.out.println("=====>>>>>>> Receive SELF-HEARTBEAT...!!!");
            restTemplate.getForObject("https://lumo-api-gateway.onrender.com/heartbeat", String.class);
        } catch (Exception e) {
            System.out.println("Ping failed: " + e.getMessage());
        }
    }
}
