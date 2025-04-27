package main

import (
	"context"
	"errors"
	"log"
	"net"

	pb "example.com/go-otel-demo/proto"
	"example.com/go-otel-demo/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

type gatewayServer struct {
	pb.UnimplementedGreeterServer
	backendClient pb.ComputeServiceClient
}

func (s *gatewayServer) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {

	err := validateRequest(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "name should be non-empty")
	}

	bReq := &pb.ComputeRequest{Payload: req.Name}
	bResp, err := s.backendClient.Compute(ctx, bReq)
	if err != nil {
		return nil, err
	}
	return &pb.HelloReply{Message: bResp.Result}, nil
}

func validateRequest(req *pb.HelloRequest) error {
	if req.GetName() == "" {
		return errors.New("empty name")
	}
	return nil
}

func TraceIDUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		resp, err := handler(ctx, req)

		if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
			md := metadata.Pairs("trace-id", sc.TraceID().String())
			grpc.SetTrailer(ctx, md)
		}
		return resp, err
	}
}

func TraceIDStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()

		err := handler(srv, ss)

		if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
			md := metadata.Pairs("trace-id", sc.TraceID().String())
			ss.SetTrailer(md)
		}
		return err
	}
}

func main() {
	ctx := context.Background()

	shutdown, err := telemetry.SetupOTelSDK(ctx, "greeter-server")
	if err != nil {
		log.Fatalf("failed to setup OTel SDK: %v", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Printf("Error shutting down OTel SDK: %v", err)
		}
	}()

	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		log.Fatalf("failed to dial backend: %v", err)
	}
	defer conn.Close()

	backendClient := pb.NewComputeServiceClient(conn)

	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	srv := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(TraceIDUnaryServerInterceptor()),
		grpc.ChainStreamInterceptor(TraceIDStreamServerInterceptor()),
	)
	pb.RegisterGreeterServer(srv, &gatewayServer{backendClient: backendClient})
	reflection.Register(srv)

	log.Println("gateway server (Greeter) listening on :50052")

	if err := srv.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
