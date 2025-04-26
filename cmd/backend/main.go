package main

import (
	"context"
	"log"
	"net"

	"google.golang.org/grpc"

	pb "example.com/go-otel-demo/proto"
)

type backendServer struct {
	pb.UnimplementedComputeServiceServer
}

func (s *backendServer) Compute(ctx context.Context, req *pb.ComputeRequest) (*pb.ComputeResponse, error) {
	return &pb.ComputeResponse{Result: "Hello " + req.Payload + "!"}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()

	pb.RegisterComputeServiceServer(grpcServer, &backendServer{})

	log.Println("▶ backend server listening on :50051")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
