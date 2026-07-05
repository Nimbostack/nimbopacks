# Python FastAPI sample

A two-endpoint FastAPI app served by uvicorn on `:8080`.

The image installs `py3-uvicorn` and `py3-fastapi` from Wolfi at runtime;
the build step uses `pip install -r requirements.txt` to validate the
exact versions during the build.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 python-fastapi-sample
```

## View

```bash
curl http://localhost:8080
curl http://localhost:8080/healthz
curl http://localhost:8080/docs    # Swagger UI
```
