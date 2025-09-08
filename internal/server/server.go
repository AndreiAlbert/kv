package server

import (
	"kv-store/internal/config"
	"kv-store/internal/node"
	pb "kv-store/proto"
	"kv-store/store"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
)

type Server struct {
	config      *config.Config
	grpcServer  *grpc.Server
	node        *node.Node
	raftManager *node.RaftManager
}

func NewServer(config *config.Config) *Server {
	return &Server{
		config: config,
	}
}

func (s *Server) Start() error {
	if err := s.config.Validate(); err != nil {
		return err
	}

	fsm := store.NewStore()
	s.node = node.New(fsm)

	s.raftManager = node.NewRaftManager(s.config, fsm)
	raftNode, err := s.raftManager.SetupRaft()
	if err != nil {
		return err
	}
	s.node.SetRaft(raftNode)
	if err := s.setupGRPCServer(); err != nil {
		return err
	}
	go s.startGRPCServer()

	if s.config.JoinAddr != "" {
		go func() {
			if err := s.raftManager.JoinCluster(); err != nil {
				log.Fatalf("Failed to join cluster: %v", err)
			}
		}()
	}
	s.waitForShutDown()
	return nil
}

func (s *Server) setupGRPCServer() error {
	s.grpcServer = grpc.NewServer()
	pb.RegisterKeyValueStoreServer(s.grpcServer, s.node)
	log.Printf("gRPC server configured on %s", s.config.GRPCAddr())
	return nil
}

func (s *Server) startGRPCServer() {
	lis, err := net.Listen("tcp", s.config.GRPCAddr())
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	log.Printf("gRPC server starting on %s", s.config.GRPCAddr())
	if err := s.grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func (s *Server) waitForShutDown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	log.Printf("Received signal %v, shutting down ...", sig)
	s.shutDown()
}

func (s *Server) shutDown() {
	log.Printf("Shutting down grpc server ...")
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}
	log.Println("Server shutdown")
}
