package node

import (
	"context"
	"kv-store/internal/config"
	pb "kv-store/proto"
	"kv-store/store"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RaftManager struct {
	config *config.Config
	fsm    *store.Store
}

func NewRaftManager(cfg *config.Config, fsm *store.Store) *RaftManager {
	return &RaftManager{
		config: cfg,
		fsm:    fsm,
	}
}

func (rm *RaftManager) SetupRaft() (*raft.Raft, error) {
	if err := os.MkdirAll(rm.config.DataDir, 0700); err != nil {
		return nil, err
	}

	// Setup Raft configuration
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(rm.config.NodeId)

	logStore, err := raftboltdb.NewBoltStore(filepath.Join(rm.config.DataDir, "logs.dat"))
	if err != nil {
		return nil, err
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(rm.config.DataDir, "stable.dat"))
	if err != nil {
		return nil, err
	}

	snapshotStore, err := raft.NewFileSnapshotStore(rm.config.DataDir, 2, os.Stderr)
	if err != nil {
		return nil, err
	}

	transport, err := raft.NewTCPTransport(rm.config.RaftAddr(), nil, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return nil, err
	}

	raftNode, err := raft.NewRaft(raftConfig, rm.fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return nil, err
	}

	if rm.config.Bootstrap {
		if err := rm.bootstrap(raftNode, raftConfig, transport); err != nil {
			return nil, err
		}
	}

	return raftNode, nil
}

func (rm *RaftManager) bootstrap(raftNode *raft.Raft, raftConfig *raft.Config, transport *raft.NetworkTransport) error {
	configuration := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raftConfig.LocalID,
				Address: transport.LocalAddr(),
			},
		},
	}

	future := raftNode.BootstrapCluster(configuration)
	return future.Error()
}

func (rm *RaftManager) JoinCluster() error {
	if rm.config.JoinAddr == "" {
		return nil
	}

	log.Printf("Attempting to join cluster at %s", rm.config.JoinAddr)

	time.Sleep(3 * time.Second)

	conn, err := grpc.NewClient(rm.config.JoinAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pb.NewKeyValueStoreClient(conn)

	_, err = client.Join(context.Background(), &pb.JoinRequest{
		NodeId:   rm.config.NodeId,
		RaftAddr: rm.config.RaftAddr(),
	})
	if err != nil {
		return err
	}

	log.Printf("Successfully joined cluster at %s", rm.config.JoinAddr)
	return nil
}
