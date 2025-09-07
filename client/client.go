package main

import (
	"context"
	pb "kv-store/proto"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewKeyValueStoreClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if len(os.Args) < 2 {
		log.Fatalf("Usage: go run client.go [get|set|delete] [key] [value]")
	}

	action := os.Args[1]
	switch action {
	case "get":
		if len(os.Args) != 3 {
			log.Fatal("Usage: go run client.go get [key]")
		}
		key := os.Args[2]
		res, err := client.Get(ctx, &pb.GetRequest{Key: key})
		if err != nil {
			log.Fatalf("Could not get value: %v", err)
		}
		log.Printf("Got value: %s", res.Value)
	case "set":
		if len(os.Args) != 4 {
			log.Fatal("Usage: go run client.go set [key] [value]")
		}
		key := os.Args[2]
		val := os.Args[3]
		_, err := client.Set(ctx, &pb.SetRequest{Key: key, Value: val})
		if err != nil {
			log.Fatalf("Could not set value %s", err)
		}
		log.Printf("Succesfully set key %s to value %s", key, val)
	case "delete":
		if len(os.Args) != 3 {
			log.Fatal("Usage: go run client.go delete [key]")
		}
		key := os.Args[2]
		_, err := client.Delete(ctx, &pb.DeleteRequest{Key: key})
		if err != nil {
			log.Fatalf("Could not delete key: %s", err)
		}
		log.Printf("Delete key %s", key)
	default:
		log.Fatal("unkown action: Use: get, set, delete")
	}
}
