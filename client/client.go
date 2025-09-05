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
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %+v", err)
	}
	defer conn.Close()
	client := pb.NewKeyValueStoreClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if len(os.Args) < 2 {
		log.Fatalf("Usage: go run client.go [set|get|delete] [key] [value]")
	}
	action := os.Args[1]
	switch action {
	case "set":
		if len(os.Args) != 4 {
			log.Fatal("Usage go run client.go set <key> <client>")
		}
		key := os.Args[2]
		value := os.Args[3]
		_, err := client.Set(ctx, &pb.SetRequest{Key: key, Value: value})
		if err != nil {
			log.Fatalf("Could not set value: %s", err)
		}
		log.Printf("Succesfully set %s to %s", key, value)
	case "get":
		if len(os.Args) != 3 {
			log.Fatal("Usage go run client.go get <key>")
		}
		key := os.Args[2]
		res, err := client.Get(ctx, &pb.GetRequest{Key: key})
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Got value: %s", res.GetValue())
	case "delete":
		if len(os.Args) != 3 {
			log.Fatal("Usage go run client.go delete <key>")
		}
		key := os.Args[2]
		client.Delete(ctx, &pb.DeleteRequest{Key: key})
		log.Printf("Delete key=%s\n", key)
	}
}
