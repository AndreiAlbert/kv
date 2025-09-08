package config

import (
	"flag"
	"fmt"
	"path/filepath"
)

type Config struct {
	GRPCPort  string
	RaftPort  string
	NodeId    string
	Bootstrap bool
	JoinAddr  string
	DataDir   string
}

func NewFromFlags() *Config {
	grpcport := flag.String("grpcport", "50051", "grpc server port")
	raftPort := flag.String("raftport", "60051", "raft port server")
	nodeId := flag.String("id", "node1", "id of the node")
	bootStrap := flag.Bool("bootstrap", false, "Bootstrap the cluster")
	joinAddr := flag.String("join", "", "Address of a node to join")
	flag.Parse()

	return &Config{
		GRPCPort:  *grpcport,
		RaftPort:  *raftPort,
		NodeId:    *nodeId,
		Bootstrap: *bootStrap,
		JoinAddr:  *joinAddr,
		DataDir:   filepath.Join("data", *nodeId),
	}
}

func (c *Config) Validate() error {
	if c.NodeId == "" {
		return fmt.Errorf("node id cannot be empty")
	}
	if c.GRPCPort == "" {
		return fmt.Errorf("grpcport cannot be empty")
	}
	if c.RaftPort == "" {
		return fmt.Errorf("raftport cannot be empty")
	}
	if c.Bootstrap && c.JoinAddr != "" {
		return fmt.Errorf("cannot bootstrap and join at the same time")
	}
	return nil
}

func (c *Config) RaftAddr() string {
	return "localhost:" + c.RaftPort
}

func (c *Config) GRPCAddr() string {
	return ":" + c.GRPCPort
}
