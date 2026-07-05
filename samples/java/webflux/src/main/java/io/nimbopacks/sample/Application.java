package io.nimbopacks.sample;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;
import reactor.core.publisher.Mono;

@SpringBootApplication
@RestController
public class Application {

    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }

    @GetMapping("/")
    public Mono<String> hello() {
        return Mono.just("Hello from nimbopacks — java/webflux sample\n");
    }

    @GetMapping("/healthz")
    public Mono<String> healthz() {
        return Mono.just("ok");
    }
}
