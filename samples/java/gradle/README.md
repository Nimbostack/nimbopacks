# Java Gradle sample

Spring Boot 3 web app built with Gradle on JDK 21. `bootJar.archiveFileName
= 'app.jar'` keeps the output filename stable so the image entrypoint can
reference `/app/app.jar` directly.

The build runs Gradle from the Wolfi `gradle` package (no Gradle wrapper
shipped) and skips the daemon for clean container builds.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 java-gradle-sample
```

## View

```bash
curl http://localhost:8080
curl http://localhost:8080/actuator/health
```
