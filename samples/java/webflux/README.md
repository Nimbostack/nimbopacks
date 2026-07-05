# Java Spring Boot WebFlux sample

Spring Boot 3 **reactive** app built with Maven on JDK 21. Unlike the
`samples/java/maven` and `samples/java/gradle` samples (which use the blocking
Spring MVC / servlet stack via `spring-boot-starter-web`), this sample uses
`spring-boot-starter-webflux` — the non-blocking reactive stack running on
**Netty**. The endpoints return `Mono<String>` rather than plain `String`.

## Jar packaging

`spring-boot-maven-plugin` repackages the application into a single executable
fat jar. With `<finalName>app</finalName>` in `pom.xml`, that jar is
`target/app.jar`, so the image cmd is the standard `["-jar", "/app/app.jar"]`.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 java-webflux-sample
```

## View

```bash
curl http://localhost:8080
curl http://localhost:8080/healthz
curl http://localhost:8080/actuator/health
```
