package main

import (
	"context"
	"flag"
	"fmt"
	pb "kv-store/proto"
	"kv-store/store"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type Node struct {
	pb.UnimplementedKeyValueStoreServer
	pb.UnimplementedReplicationServer
	mu               sync.RWMutex
	store            *store.Store
	isLeader         bool
	replicationPeers map[string]pb.ReplicationClient
}

func NewNode(isLeader bool) *Node {
	return &Node{
		store:            store.NewStore(),
		isLeader:         isLeader,
		replicationPeers: make(map[string]pb.ReplicationClient),
	}
}

func (n *Node) connectToPeers(peerAddrs []string) {
	for _, addr := range peerAddrs {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Failed to connect to peer %s: %v", addr, err)
			continue
		}
		client := pb.NewReplicationClient(conn)
		n.replicationPeers[addr] = client
		log.Printf("Connected to peer %s", addr)
	}
}

func (n *Node) Set(ctx context.Context, req *pb.SetRequest) (*pb.SetResponse, error) {
	if !n.isLeader {
		return nil, status.Error(codes.FailedPrecondition, "this node is not the leader")
	}
	n.store.Set(req.Key, req.Value)
	log.Printf("LEADER: Set key=%s, value=%s", req.Key, req.Value)
	for peerAddr, peerClient := range n.replicationPeers {
		go func(addr string, client pb.ReplicationClient) {
			repl_req := &pb.ReplicateRequest{Key: req.Key, Value: req.Value, Operation: pb.ReplicateRequest_SET}
			_, err := client.Replicate(context.Background(), repl_req)
			if err != nil {
				log.Printf("Failed to replicate to peer %s: %v", addr, err)
			}
		}(peerAddr, peerClient)
	}
	return &pb.SetResponse{Ok: true}, nil
}

func (n *Node) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if !n.isLeader {
		return nil, status.Error(codes.FailedPrecondition, "this node is not leader")
	}
	n.store.Delete(req.Key)
	log.Printf("LEADER: Delete key=%s", req.Key)
	for peerAddr, peerClient := range n.replicationPeers {
		go func(addr string, client pb.ReplicationClient) {
			repl_req := &pb.ReplicateRequest{Key: req.Key, Value: "", Operation: pb.ReplicateRequest_DELETE}
			_, err := client.Replicate(context.Background(), repl_req)
			if err != nil {
				log.Printf("failec to replicate to peer %s: %v", addr, err)
			}
		}(peerAddr, peerClient)
	}
	return &pb.DeleteResponse{Ok: true}, nil
}

func (n *Node) Replicate(ctx context.Context, req *pb.ReplicateRequest) (*pb.ReplicateResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	switch req.Operation {
	case pb.ReplicateRequest_DELETE:
		n.store.Delete(req.Key)
		log.Printf("Delete replicate on for key=%s\n", req.Key)
		return &pb.ReplicateResponse{Ok: true}, nil
	case pb.ReplicateRequest_SET:
		n.store.Set(req.Key, req.Value)
		log.Printf("Replicate set for key=%s with value=%s\n", req.Key, req.Value)
		return &pb.ReplicateResponse{Ok: true}, nil
	default:
		panic(fmt.Sprintf("unexpected proto.ReplicateRequest_OpType: %#v", req.Operation))
	}
}

func (n *Node) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	value, ok := n.store.Get(req.Key)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "key %s not found", req.Key)
	}
	return &pb.GetResponse{Value: value}, nil
}

func main() {
	port := flag.String("port", "50051", "Server port")
	isLeader := flag.Bool("leader", false, "Is this node the leader?")
	peerList := flag.String("peers", "", "Comma-separated list of peer addresses (e.g localhost:50052,localhost:50053)")
	flag.Parse()

	addr := "localhost:" + *port
	lis, err := net.Listen("tcp", ":"+*port)
	if err != nil {
		log.Fatal("could not listen")
	}
	node := NewNode(*isLeader)
	go func() {
		time.Sleep(5 * time.Second)
		if *peerList != "" {
			peers := strings.Split(*peerList, ",")
			node.connectToPeers(peers)
		}
	}()

	grpcServer := grpc.NewServer()
	pb.RegisterKeyValueStoreServer(grpcServer, node)
	pb.RegisterReplicationServer(grpcServer, node)
	log.Printf("Server starting on %s. Leader: %v", addr, *isLeader)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
