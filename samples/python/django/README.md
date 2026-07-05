# Python Django sample

A trimmed-down Django project served by gunicorn — no database, no admin,
just two URL handlers wired through `config/urls.py`. Enough to exercise
the `python-django` template end to end without dragging in everything a
full project would need.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 python-django-sample
```

## View

```bash
curl http://localhost:8080
curl http://localhost:8080/healthz
```
