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
            restTemplate.getForObject("https://glad-candida-lumo-user-svc-ee901669.koyeb.app/heartbeat", String.class);
        } catch (Exception e) {
            System.out.println("Ping failed: " + e.getMessage());
        }
    }
}
