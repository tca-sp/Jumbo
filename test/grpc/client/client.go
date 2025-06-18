package main

import (
	"context"
	"flag"
	"log"
	"time"

	pb "jumbo/test/grpc/kvstore" // 替换为你的包路径

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	serverAddr := flag.String("addr", "localhost:50051", "The server address in the format of host:port")
	op := flag.String("op", "get", "Operation 'put' or 'get'")
	key := flag.String("key", "", "Key for the operation")
	value := flag.String("value", "", "Value for put operation")
	flag.Parse()

	// 建立连接
	conn, err := grpc.Dial(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewKeyValueStoreClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch *op {
	case "put":
		if *key == "" || *value == "" {
			log.Fatal("key and value must be specified for put operation")
		}
		r, err := c.Put(ctx, &pb.PutRequest{Key: *key, Value: *value})
		if err != nil {
			log.Fatalf("could not put: %v", err)
		}
		log.Printf("Put result: %v", r.Success)
	case "get":
		if *key == "" {
			log.Fatal("key must be specified for get operation")
		}
		r, err := c.Get(ctx, &pb.GetRequest{Key: *key})
		if err != nil {
			log.Fatalf("could not get: %v", err)
		}
		if r.Found {
			log.Printf("Get result: %s = %s", *key, r.Value)
		} else {
			log.Printf("Key %s not found", *key)
		}
	default:
		log.Fatal("unknown operation, use 'put' or 'get'")
	}
}
