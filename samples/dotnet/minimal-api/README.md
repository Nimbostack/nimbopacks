# .NET minimal API sample

A single-file ASP.NET Core minimal API on .NET 8.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 dotnet-minimal-api-sample
```

## View

```bash
curl http://localhost:8080
curl http://localhost:8080/healthz
```
