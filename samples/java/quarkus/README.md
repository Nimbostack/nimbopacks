# Java Quarkus sample

Minimal Quarkus 3 REST app (Quarkus REST / RESTEasy Reactive) built with Maven
on JDK 21.

## Jar packaging

Quarkus' default `mvn package` produces a `target/quarkus-app/` directory layout
(`quarkus-run.jar` plus a `lib/` folder) rather than a single self-contained jar.
The nimbopacks java pack copies jars out of `JAR_DIR` and runs one jar by path,
so this sample switches Quarkus to **uber-jar** packaging:

```properties
quarkus.package.jar.type=uber-jar   # Quarkus 3.x property (was quarkus.package.type in <3.x)
quarkus.package.output-name=app
```

With `output-name=app`, the uber-jar is emitted as `target/app-runner.jar`. Quarkus
still also writes a thin `target/app.jar`, which is **not** self-contained. Both jars
end up in `/app` inside the image, so `image.cmd` points explicitly at the runnable
one: `["-jar", "/app/app-runner.jar"]`.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 java-quarkus-sample
```

## View

```bash
curl http://localhost:8080
curl http://localhost:8080/healthz
```
