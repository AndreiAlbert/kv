package node

import (
	"context"
	"encoding/json"
	pb "kv-store/proto"
	"kv-store/store"
	"log"
	"time"

	"github.com/hashicorp/raft"
	"google.golang.org/grpc/codes"
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

func (n *Node) Set(ctx context.Context, req *pb.SetRequest) (*pb.SetResponse, error) {
	if n.raft.State() != raft.Leader {
		return nil, status.Error(codes.FailedPrecondition, "not leader")
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
		return nil, status.Error(codes.FailedPrecondition, "not leader")
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
