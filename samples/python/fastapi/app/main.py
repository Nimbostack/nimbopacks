from fastapi import FastAPI

app = FastAPI(title="nimbopacks fastapi sample")


@app.get("/")
def root() -> dict[str, str]:
    return {"message": "Hello from nimbopacks — python/fastapi sample"}


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"status": "ok"}
