package main

import (
	"context"
	"flag"
	pb "kv-store/proto"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	serverAddr := flag.String("server", "localhost:50051", "server address to connect to")
	flag.Parse()

	args := flag.Args()
	if len(args) < 2 {
		log.Fatalf("Usage: client [-server=address] [get|set|delete] [key] [value]")
	}

	conn, err := grpc.NewClient(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewKeyValueStoreClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	log.Printf("Connecting to server: %s", *serverAddr)

	action := args[0]
	switch action {
	case "get":
		if len(args) != 2 {
			log.Fatal("Usage: client [-server=address] get [key]")
		}
		key := args[1]
		res, err := client.Get(ctx, &pb.GetRequest{Key: key})
		if err != nil {
			log.Fatalf("Could not get value: %v", err)
		}
		log.Printf("Got value: %s", res.Value)
	case "set":
		if len(args) != 3 {
			log.Fatal("Usage: client [-server=address] set [key] [value]")
		}
		key := args[1]
		val := args[2]
		_, err := client.Set(ctx, &pb.SetRequest{Key: key, Value: val})
		if err != nil {
			log.Fatalf("Could not set value: %s", err)
		}
		log.Printf("Successfully set key %s to value %s", key, val)
	case "delete":
		if len(args) != 2 {
			log.Fatal("Usage: client [-server=address] delete [key]")
		}
		key := args[1]
		_, err := client.Delete(ctx, &pb.DeleteRequest{Key: key})
		if err != nil {
			log.Fatalf("Could not delete key: %s", err)
		}
		log.Printf("Deleted key %s", key)
	default:
		log.Fatal("Unknown action: Use: get, set, delete")
	}
}
