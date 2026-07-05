package io.nimbopacks.sample;

import io.micronaut.http.MediaType;
import io.micronaut.http.annotation.Controller;
import io.micronaut.http.annotation.Get;
import io.micronaut.http.annotation.Produces;

@Controller("/")
public class HelloController {

    @Get
    @Produces(MediaType.TEXT_PLAIN)
    public String hello() {
        return "Hello from nimbopacks — java/micronaut sample\n";
    }

    @Get("/healthz")
    @Produces(MediaType.TEXT_PLAIN)
    public String healthz() {
        return "ok";
    }
}
