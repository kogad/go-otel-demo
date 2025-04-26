package main

import (
	"context"
	"log"
	"net"

	pb "example.com/go-otel-demo/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

type gatewayServer struct {
	pb.UnimplementedGreeterServer
	backendClient pb.ComputeServiceClient
}

func (s *gatewayServer) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
	bReq := &pb.ComputeRequest{Payload: req.Name}
	bResp, err := s.backendClient.Compute(ctx, bReq)
	if err != nil {
		return nil, err
	}
	return &pb.HelloReply{Message: bResp.Result}, nil
}

func main() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to dial backend: %v", err)
	}
	defer conn.Close()

	backendClient := pb.NewComputeServiceClient(conn)

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer()
	pb.RegisterGreeterServer(srv, &gatewayServer{backendClient: backendClient})
	reflection.Register(srv)

	log.Println("gateway server (Greeter) listening on :50052")

	if err := srv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
