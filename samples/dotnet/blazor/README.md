# .NET Blazor sample

A minimal .NET 8 Blazor Web App using interactive server-side rendering
(Blazor Server). It serves a single `Home` page with an interactive counter.
Listens on port 8080.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 dotnet-blazor-sample
```

## View

Open the app in a browser:

```bash
curl http://localhost:8080/
# or visit http://localhost:8080/ and click the counter button
```
