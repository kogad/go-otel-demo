package main

import (
	"context"
	"errors"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"example.com/go-otel-demo/logger"
	pb "example.com/go-otel-demo/proto"
	"example.com/go-otel-demo/telemetry"
)

const name = "example.com/go-otel-demo"

var (
	tracer = otel.Tracer(name)
	meter  = otel.Meter(name)
	//logger  = otelslog.NewLogger(name)
)

type backendServer struct {
	pb.UnimplementedComputeServiceServer
}

func (s *backendServer) Compute(ctx context.Context, req *pb.ComputeRequest) (*pb.ComputeResponse, error) {
	slog.InfoContext(ctx, "start Compute", "request", req)
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("req.payload", req.Payload))

	msg, err := generateMessage(ctx, req.Payload)
	if err != nil {
		slog.ErrorContext(ctx, "failed to generate message", "err", err)
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &pb.ComputeResponse{Result: msg}, nil
}

func generateMessage(ctx context.Context, s string) (string, error) {
	ctx, span := tracer.Start(ctx, "generateMessage")
	defer span.End()

	if s == "heavy" {
		span.AddEvent("starting heavy computation ")
		time.Sleep(2 * time.Second)
		span.AddEvent("finish heavy computation")
	}

	if s == "internal error" {
		return "", errors.New("error")
	}

	return "Hello " + s + "!", nil
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

	f, err := os.OpenFile("backend.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		panic(err)
	}

	w := io.MultiWriter(os.Stdout, f)

	jsonHandler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(logger.NewTraceHandler(jsonHandler))
	slog.SetDefault(logger)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	pb.RegisterComputeServiceServer(grpcServer, &backendServer{})

	slog.InfoContext(ctx, "backend server listening on :50051")

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
