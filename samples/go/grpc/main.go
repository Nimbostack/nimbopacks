// Minimal gRPC server demonstrating the `go-grpc` template.
//
// Registers two services that ship with grpc-go and need no .proto generation
// to use:
//
//   - grpc.health.v1.Health        (standard gRPC health protocol)
//   - grpc.reflection.v1.ServerReflection
//
// Try it with grpcurl:
//
//   grpcurl -plaintext localhost:50051 list
//   grpcurl -plaintext localhost:50051 grpc.health.v1.Health/Check
package main

import (
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	addr := ":" + envOr("PORT", "50051")

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen %s: %v", addr, err)
	}

	s := grpc.NewServer()

	healthSvc := health.NewServer()
	healthSvc.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, healthSvc)

	reflection.Register(s)

	log.Printf("gRPC listening on %s", addr)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
