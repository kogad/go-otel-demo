package main

import (
	"context"
	"log"
	"net"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	pb "example.com/go-otel-demo/proto"
	"example.com/go-otel-demo/telemetry"
)

type backendServer struct {
	pb.UnimplementedComputeServiceServer
}

func (s *backendServer) Compute(ctx context.Context, req *pb.ComputeRequest) (*pb.ComputeResponse, error) {
	return &pb.ComputeResponse{Result: "Hello " + req.Payload + "!"}, nil
}

func main() {
	ctx := context.Background()

	shutdown, err := telemetry.SetupOTelSDK(ctx, "backend-server")
	if err != nil {
		log.Fatalf("failed to setup OTel SDK: %v", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Printf("Error shutting down OTel SDK: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	pb.RegisterComputeServiceServer(grpcServer, &backendServer{})

	log.Println("▶ backend server listening on :50051")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
