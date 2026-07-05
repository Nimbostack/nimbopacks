# .NET gRPC sample

ASP.NET Core 8 gRPC service exposing a `Greeter` service with a `SayHello`
RPC (defined in `Protos/greet.proto`). Kestrel is configured for HTTP/2 in
plaintext (h2c) on port 8080 so the service works without TLS inside a
container.

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 8080:8080 dotnet-grpc-sample
```

## View

Call the service with [grpcurl](https://github.com/fullstorydev/grpcurl)
(use `-plaintext` because the container serves h2c without TLS):

```bash
grpcurl -plaintext -d '{"name": "world"}' \
  localhost:8080 greet.Greeter/SayHello
```
