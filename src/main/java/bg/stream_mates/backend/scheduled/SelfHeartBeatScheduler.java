package bg.stream_mates.backend.scheduled;

import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestTemplate;

@Component
public class SelfHeartBeatScheduler {

    private final RestTemplate restTemplate = new RestTemplate();

    @Scheduled(cron = "0 0 9 * * ?") // всеки ден в 9ч.
    public void heartBeatSelf() {
        try {
            restTemplate.getForObject("https://user-svc-url/heartbeat", String.class);
        } catch (Exception e) {
            System.out.println("Ping failed: " + e.getMessage());
        }
    }
}
