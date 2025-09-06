package main

import (
	"context"
	"encoding/json"
	"flag"
	pb "kv-store/proto"
	"kv-store/store"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/raft-boltdb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type Node struct {
	pb.UnimplementedKeyValueStoreServer
	raft *raft.Raft
	fsm  *store.Store
}

func NewNode(fsm *store.Store) *Node {
	return &Node{
		fsm: fsm,
	}
}

func (n *Node) Set(ctx context.Context, req *pb.SetRequest) (*pb.SetResponse, error) {
	if n.raft.State() != raft.Leader {
		return nil, status.Error(codes.FailedPrecondition, "not leader")
	}

	cmd := &store.Command{
		Op:    "set",
		Key:   req.Key,
		Value: req.Value,
	}
	b, err := json.Marshal(cmd)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal command")
	}
	future := n.raft.Apply(b, 500*time.Millisecond)
	if err := future.Error(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to apply command: %s", err)
	}
	return &pb.SetResponse{Ok: true}, nil
}

func (n *Node) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if n.raft.State() != raft.Leader {
		return nil, status.Error(codes.FailedPrecondition, "not leader")
	}

	cmd := &store.Command{
		Op:  "delete",
		Key: req.Key,
	}
	b, err := json.Marshal(cmd)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal command")
	}
	future := n.raft.Apply(b, 500*time.Millisecond)
	if err := future.Error(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to apply command: %s", err)
	}
	return &pb.DeleteResponse{Ok: true}, nil
}

func (n *Node) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	val, ok := n.fsm.Get(req.Key)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "key %s not found", req.Key)
	}
	return &pb.GetResponse{Value: val}, nil
}

func (n *Node) Join(ctx context.Context, req *pb.JoinRequest) (*pb.JoinResponse, error) {
	log.Printf("received join request for node %s at %s", req.NodeId, req.RaftAddr)
	if n.raft.State() != raft.Leader {
		return nil, status.Error(codes.FailedPrecondition, "not the leader")
	}
	future := n.raft.AddVoter(raft.ServerID(req.NodeId), raft.ServerAddress(req.RaftAddr), 0, 0)
	if err := future.Error(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to add voter: %s", err)
	}
	return &pb.JoinResponse{Ok: true}, nil
}

func main() {
	grpcPort := flag.String("grpcport", "50051", "gRPC server port")
	raftPort := flag.String("raftport", "60051", "Raft server port")
	nodeID := flag.String("id", "node1", "Node ID")
	bootstrap := flag.Bool("bootstrap", false, "Bootstrap the cluster")
	joinAddr := flag.String("join", "", "Address of a node to join")
	flag.Parse()

	dataDir := "data/" + *nodeID
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		log.Fatalf("failed to create data dir: %s", err)
	}

	fsm := store.NewStore()

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(*nodeID)

	logStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "logs.dat"))
	if err != nil {
		log.Fatalf("failed to create log store: %s", err)
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(dataDir, "stable.dat"))
	if err != nil {
		log.Fatalf("failed to create stable store: %s", err)
	}

	snapshotStore, err := raft.NewFileSnapshotStore(dataDir, 2, os.Stderr)
	if err != nil {
		log.Fatalf("failed to create snapshot store: %s", err)
	}

	raftAddr := "localhost:" + *raftPort
	transport, err := raft.NewTCPTransport(raftAddr, nil, 3, 10*time.Second, os.Stderr)
	if err != nil {
		log.Fatalf("failed to create transport: %s", err)
	}

	raftNode, err := raft.NewRaft(config, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		log.Fatalf("failed to create raft node: %s", err)
	}

	if *bootstrap {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		raftNode.BootstrapCluster(configuration)
	}

	node := NewNode(fsm)
	node.raft = raftNode

	lis, err := net.Listen("tcp", ":"+*grpcPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterKeyValueStoreServer(grpcServer, node)
	log.Printf("grpc server starting on port %s", *grpcPort)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %s", err)
		}
	}()

	if *joinAddr != "" {
		go func() {
			time.Sleep(3 * time.Second)
			conn, err := grpc.NewClient(*joinAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Fatalf("failed to connect to join addr: %v", err)
			}
			defer conn.Close()
			client := pb.NewKeyValueStoreClient(conn)
			_, err = client.Join(context.Background(), &pb.JoinRequest{
				NodeId:   *nodeID,
				RaftAddr: raftAddr,
			})
			if err != nil {
				log.Fatalf("failed to join cluster: %v", err)
			}
			log.Printf("succesfully joined cluster at %s", *joinAddr)
		}()
	}
	select {}
}
