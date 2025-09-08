package node

import (
	"context"
	"encoding/json"
	pb "kv-store/proto"
	"kv-store/store"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/raft"
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

func New(fsm *store.Store) *Node {
	return &Node{
		fsm: fsm,
	}
}

func (n *Node) SetRaft(r *raft.Raft) {
	n.raft = r
}

// getLeaderGRPCAddr converts a raft address to grpc address
// assumes gRPC port is raft port - 10000
func (n *Node) getLeaderGRPCAddr() (string, error) {
	leaderAddr, leaderId := n.raft.LeaderWithID()
	if leaderAddr == "" {
		return "", status.Error(codes.Unavailable, "no leader available")
	}
	log.Printf("Current leader: %s at %s", leaderId, leaderAddr)

	addrStr := string(leaderAddr)
	parts := strings.Split(addrStr, ":")
	if len(parts) != 2 {
		return "", status.Error(codes.Internal, "invalid leader address format")
	}

	host := parts[0]
	portStr := parts[1]

	raftPort, err := strconv.Atoi(portStr)
	if err != nil {
		if strings.Contains(addrStr, ":60051") {
			return strings.Replace(addrStr, ":60051", ":50051", 1), nil
		}
		if strings.Contains(addrStr, ":60052") {
			return strings.Replace(addrStr, ":60052", ":50052", 1), nil
		}
		if strings.Contains(addrStr, ":60053") {
			return strings.Replace(addrStr, ":60053", ":50053", 1), nil
		}
		return host + ":50051", nil
	}
	grpcPort := raftPort - 10000
	if grpcPort <= 0 {
		return "", status.Error(codes.Internal, "calculated gRPC port is invalid")
	}
	return host + ":" + strconv.Itoa(grpcPort), nil
}

func (n *Node) redirectToLeader(ctx context.Context, operation string, key string, value string) error {
	leaderAddr, err := n.getLeaderGRPCAddr()
	if err != nil {
		return err
	}
	log.Printf("Redirecting %s request to leader at %s", operation, leaderAddr)

	conn, err := grpc.NewClient(leaderAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return status.Errorf(codes.Internal, "failed to connect to leader: %v", err)
	}
	defer conn.Close()

	client := pb.NewKeyValueStoreClient(conn)

	redirectCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()

	switch operation {
	case "set":
		_, err = client.Set(redirectCtx, &pb.SetRequest{Key: key, Value: value})
	case "delete":
		_, err = client.Delete(redirectCtx, &pb.DeleteRequest{Key: key})
	default:
		return status.Error(codes.Internal, "unkown operation")
	}
	return err
}

func (n *Node) Set(ctx context.Context, req *pb.SetRequest) (*pb.SetResponse, error) {
	if n.raft.State() != raft.Leader {
		if err := n.redirectToLeader(ctx, "set", req.Key, req.Value); err != nil {
			return nil, err
		}
		return &pb.SetResponse{Ok: true}, nil
	}
	cmd := store.Command{
		Op:    "set",
		Key:   req.Key,
		Value: req.Value,
	}

	b, err := json.Marshal(cmd)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal command")
	}

	future := n.raft.Apply(b, 500*time.Millisecond)
	if err = future.Error(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to apply commanf: %s", err)
	}
	return &pb.SetResponse{Ok: true}, nil
}

func (n *Node) Delete(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	if n.raft.State() != raft.Leader {
		if err := n.redirectToLeader(ctx, "delete", req.Key, ""); err != nil {
			return nil, err
		}
		return &pb.DeleteResponse{Ok: true}, nil
	}
	cmd := store.Command{
		Op:  "delete",
		Key: req.Key,
	}

	b, err := json.Marshal(cmd)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to marshal command")
	}

	future := n.raft.Apply(b, 500*time.Millisecond)
	if err = future.Error(); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to apply commanf: %s", err)
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
