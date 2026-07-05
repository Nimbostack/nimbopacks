# Java Micronaut sample

Minimal Micronaut 4 app (Netty HTTP server) built with Maven on JDK 21.

## Jar packaging

The Micronaut Maven plugin's `package` goal builds a single executable shaded
("fat") jar at `target/<finalName>.jar`. This sample sets `<finalName>app</finalName>`
in `pom.xml`, so the runnable jar is `target/app.jar` and the image cmd is the
standard `["-jar", "/app/app.jar"]`.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 java-micronaut-sample
```

## View

```bash
curl http://localhost:8080
curl http://localhost:8080/healthz
```
