package main

import (
	"context"
	"log"
	"net"

	pb "jumbo/test/grpc/kvstore" // 替换为你的包路径

	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedKeyValueStoreServer
}

func (s *server) Put(ctx context.Context, req *pb.PutRequest) (*pb.PutResponse, error) {

	return &pb.PutResponse{Success: true}, nil
}

func (s *server) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {

	return &pb.GetResponse{Value: "value", Found: true}, nil
}

func main() {

	// 启动gRPC服务器
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterKeyValueStoreServer(s, &server{})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
