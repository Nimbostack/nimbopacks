# Go gRPC sample

A minimal gRPC server. To keep the sample readable and free of `protoc`
boilerplate, it registers two services that ship with `grpc-go` and need no
`.proto` generation:

- `grpc.health.v1.Health` — the standard gRPC health-check protocol
- `grpc.reflection.v1.ServerReflection` — exposes the schema at runtime

## Build

```bash
nimbopacks build
```

## Run

```bash
docker run --rm -p 50051:50051 go-grpc-sample
```

## View

With [`grpcurl`](https://github.com/fullstorydev/grpcurl):

```bash
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
```
