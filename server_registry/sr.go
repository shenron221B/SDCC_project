package server_registry

import (
	pbNode "SDCC/node/registry"
	pb "SDCC/server_registry/registry"
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
	"sync"
	"time"
)

type server struct {
	pb.UnimplementedRegistryServer
	mu      sync.Mutex
	nodeMap map[string]string
}

func NewServer() *server {
	return &server{
		nodeMap: make(map[string]string),
	}
}

func (s *server) RegisterNode(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodeMap[req.Name] = req.Address
	log.Printf("Node registered: %s at %s", req.Name, req.Address)
	log.Printf("Current nodeMap: %v", s.nodeMap)
	go s.notifyAllNodes()
	return &pb.RegisterResponse{Success: true}, nil
}

func (s *server) GetNodeAddress(ctx context.Context, req *pb.NodeRequest) (*pb.NodeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	address, exists := s.nodeMap[req.Name]
	if !exists {
		return &pb.NodeResponse{Address: ""}, nil
	}
	return &pb.NodeResponse{Address: address}, nil
}

func (s *server) GetAllNodes(ctx context.Context, req *pb.Empty) (*pb.NodeListResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nodes := []*pb.Node{}
	for name, address := range s.nodeMap {
		nodes = append(nodes, &pb.Node{Name: name, Address: address})
	}
	return &pb.NodeListResponse{Nodes: nodes}, nil
}

func StartRegistryServer(port string) {
	// initialize tcp listener
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterRegistryServer(grpcServer, NewServer())
	reflection.Register(grpcServer)

	log.Printf("Registry server listening on %s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func (s *server) notifyAllNodes() {
	for _, address := range s.nodeMap {
		conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Failed to connect to node at %s: %v", address, err)
			continue
		}
		defer conn.Close()

		client := pbNode.NewNodeClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		nodes := []*pbNode.NodeInfo{}
		for name, addr := range s.nodeMap {
			nodes = append(nodes, &pbNode.NodeInfo{Name: name, Address: addr})
		}

		_, err = client.UpdatePeers(ctx, &pbNode.NodeListResponse{Nodes: nodes})
		if err != nil {
			log.Printf("Failed to notify node at %s: %v", address, err)
		}
	}
}
