# Java Maven sample

Spring Boot 3 web app built with Maven on JDK 21. The packaged JAR is named
`app.jar` (via `<finalName>app</finalName>` in `pom.xml`) so the image
entrypoint can reference a stable path.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 java-maven-sample
```

## View

```bash
curl http://localhost:8080
curl http://localhost:8080/actuator/health
```
