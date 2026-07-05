# .NET solution (monorepo) sample

A real `.sln` with two projects packaged into **one image**:

- **`MyApp.Api`** — ASP.NET Core minimal API, listens on `:8080`.
- **`MyApp.Worker`** — `IHostedService` background worker that logs a
  heartbeat every 10 seconds.

This is the canonical nimbopacks **monorepo** pattern:
`build.command` restores the solution once, then each entry in `artifacts:`
runs its own `dotnet publish` and lands in `/app/<name>` inside the image.

## Layout

```
dotnet/solution/
├── MyApp.sln
├── nimpack.yaml
└── src/
    ├── MyApp.Api/
    │   ├── MyApp.Api.csproj
    │   ├── Program.cs
    │   └── appsettings.json
    └── MyApp.Worker/
        ├── MyApp.Worker.csproj
        ├── Program.cs
        ├── Heartbeat.cs
        └── appsettings.json
```

## Build

```bash
nimbopacks build
```

## Run

The default entrypoint runs the API:

```bash
docker run --rm -p 8080:8080 dotnet-solution-sample
curl http://localhost:8080
curl http://localhost:8080/healthz
```

To run the worker out of the same image, override the command:

```bash
docker run --rm dotnet-solution-sample dotnet /app/worker/MyApp.Worker.dll
```

## Adding more projects

Add a new directory under `src/`, register it in `MyApp.sln`, and add a new
entry to `artifacts:` in `nimpack.yaml`. No other files need to change.
