# .NET Web API sample

ASP.NET Core 8 Web API with a single `GreetingsController`. Uses the
controller-based model rather than the minimal-API style.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 dotnet-webapi-sample
```

## View

```bash
curl http://localhost:8080/greetings
curl http://localhost:8080/greetings/world
curl http://localhost:8080/healthz
```
