# .NET Worker Service sample

A .NET 8 Worker Service (`Microsoft.NET.Sdk.Worker`) running a single
`BackgroundService` that logs a heartbeat every 5 seconds. It is not a web
app, so it exposes no ports and only needs the base .NET runtime (no ASP.NET
runtime).

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm --name worker-sample dotnet-worker-sample
```

## View

The worker logs to stdout. Tail the logs:

```bash
docker logs -f worker-sample
```
