package io.nimbopacks.sample;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@SpringBootApplication
@RestController
public class Application {

    public static void main(String[] args) {
        SpringApplication.run(Application.class, args);
    }

    @GetMapping("/")
    public String hello() {
        return "Hello from nimbopacks — java/maven sample\n";
    }

    @GetMapping("/healthz")
    public String healthz() {
        return "ok\n";
    }
}
