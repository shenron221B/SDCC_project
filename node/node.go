package node

import (
	chandyLamport "SDCC/Chandy_Lamport"
	"SDCC/node/configNode"
	"context"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	pbNode "SDCC/node/registry"
	pbRegistry "SDCC/server_registry/registry"
)

type NodeServer struct {
	pbNode.UnimplementedNodeServer
	mu                  sync.Mutex
	name                string
	balance             int32
	registryAddress     string
	peers               map[string]string
	chandyLamportServer *chandyLamport.ChandyLamportServer
	predecessor         string
	successor           string
}

type NodeConfig struct {
	Address         string
	Name            string
	Balance         string
	RegistryAddress string
}

// NewNodeServer creates a new NodeServer instance
func NewNodeServer(name string, balance int32, registryAddress string, cls *chandyLamport.ChandyLamportServer) *NodeServer {
	ns := &NodeServer{
		name:                name,
		balance:             balance,
		peers:               make(map[string]string),
		registryAddress:     registryAddress,
		chandyLamportServer: cls,
	}
	log.Printf("NodeServer initialized: Name=%s, Balance=%d, RegistryAddress=%s, Peers=%v",
		name, balance, registryAddress, ns.peers)
	return ns
}

func (s *NodeServer) VerifyTransaction(ctx context.Context, req *pbNode.TransactionVerificationRequest) (*pbNode.TransactionVerificationResponse, error) {
	log.Printf("VerifyTransaction called on %s...", s.name)
	time.Sleep(2 * time.Second)

	// verify if the node must be record its state
	if s.chandyLamportServer.SeenMarkerForTheFirstTime[s.name] {
		// record state
		s.chandyLamportServer.LocalState = "VerifyTransaction - " + "verifications in progress for request: TransferMoney to " + req.TargetNode + " for " + strconv.Itoa(int(req.Amount)) + "$"
		// update version of snapshot
		s.chandyLamportServer.Version[s.name]++
		s.chandyLamportServer.SaveStateToFile()
	}

	// verify if node know the receiver
	if _, known := s.peers[req.TargetNode]; known {
		req.ConfidenceScore++
		log.Printf("%s knows the target node: %s", s.name, req.TargetNode)
	}

	// verify if exist another node that it's not the receiver
	if s.successor != "" && s.successor != req.TargetNode {

		// verify if the node must be record its state
		if s.chandyLamportServer.Recording[s.name] {
			// update state
			s.chandyLamportServer.LocalState = "VerifyTransaction - " + "transaction verified; actual confidence score: " + strconv.Itoa(int(req.ConfidenceScore))
			s.chandyLamportServer.Version[s.name]++
			s.chandyLamportServer.SaveStateToFile()
		}

		log.Printf("%s calls AnotherVerification on successor: %s", s.name, s.successor)

		// search the address of successor
		successorAddr, exists := s.peers[s.successor]
		if !exists || successorAddr == "" {
			return nil, fmt.Errorf("addess of successor not found for call AnotherVerification")
		}

		conn, err := grpc.Dial(successorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("error to connect with successor %s for call AnotherVerification: %v", s.successor, err)
		}
		defer conn.Close()
		time.Sleep(2 * time.Second)

		client := pbNode.NewNodeClient(conn)

		s.SendMarkerToOutgoingChannels(s.name)
		// call AnotherVerification on successor
		resp, err := client.AnotherVerification(ctx, req)

		if err != nil {
			return nil, fmt.Errorf("error to call AnotherVerification on successor %s: %v", s.successor, err)
		}
		return resp, nil
	}

	// this node is the last one before receiver -> send the amount
	if s.successor == req.TargetNode {

		log.Printf("this node is the last one before receiver! Sending amount...")

		peerAddr := s.peers[s.predecessor]
		conn, err := grpc.Dial(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Failed to connect to peer-1 for approval: %v", err)
			return &pbNode.TransactionVerificationResponse{Known: false, ConfidenceScore: req.ConfidenceScore}, nil
		}
		defer conn.Close()

		client := pbNode.NewNodeClient(conn)
		approvalReq := &pbNode.ApprovalRequest{
			Sender:          req.NodeName,
			Receiver:        req.TargetNode,
			Amount:          req.Amount,
			ConfidenceScore: req.ConfidenceScore,
		}

		approvalResp, err := client.RequestApproval(ctx, approvalReq)
		if err != nil || !approvalResp.Approved {
			log.Printf("Approval denied or failed for peer-1: %v", err)
			return &pbNode.TransactionVerificationResponse{Known: false, ConfidenceScore: req.ConfidenceScore}, nil
		}

		// verify if the node must be record its state
		if s.chandyLamportServer.Recording[s.name] {
			// update state
			s.chandyLamportServer.LocalState = "VerifyTransaction - " + "transaction verified; sent " + strconv.Itoa(int(req.Amount)) + "$ to " + req.TargetNode
			s.chandyLamportServer.Version[s.name]++
			s.chandyLamportServer.SaveStateToFile()
			s.SendMarkerToOutgoingChannels(s.name)
		}

		time.Sleep(1 * time.Second)

		if req.ConfidenceScore == int32(len(s.peers)-1) {
			finalPeerAddr, ok := s.peers[req.TargetNode]
			if ok {

				conn, err := grpc.DialContext(ctx, finalPeerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					return &pbNode.TransactionVerificationResponse{Known: false, ConfidenceScore: req.ConfidenceScore}, err
				}
				defer conn.Close()
				time.Sleep(2 * time.Second)

				client := pbNode.NewNodeClient(conn)
				updateReq := &pbNode.UpdateBalanceRequest{Amount: req.Amount}
				updateResp, err := client.UpdateBalance(ctx, updateReq)
				if err != nil || !updateResp.Success {
					return &pbNode.TransactionVerificationResponse{Known: false, ConfidenceScore: req.ConfidenceScore}, err
				}

				log.Printf("sending the amount...")

				return &pbNode.TransactionVerificationResponse{Known: true, ConfidenceScore: req.ConfidenceScore}, nil
			}
		} else {
			return &pbNode.TransactionVerificationResponse{Known: false, ConfidenceScore: req.ConfidenceScore}, nil
		}
	}

	return &pbNode.TransactionVerificationResponse{Known: req.ConfidenceScore > 0, ConfidenceScore: req.ConfidenceScore}, nil
}

func (s *NodeServer) AnotherVerification(ctx context.Context, req *pbNode.TransactionVerificationRequest) (*pbNode.TransactionVerificationResponse, error) {
	log.Printf("AnotherVerification called on node %s...", s.name)

	// verify if the node must be record its state
	if s.chandyLamportServer.SeenMarkerForTheFirstTime[s.name] {
		// record state
		s.chandyLamportServer.LocalState = "AnotherVerification - " + "verifications in progress for request: TransferMoney to " + req.TargetNode + " for " + strconv.Itoa(int(req.Amount)) + "$"
		s.chandyLamportServer.Version[s.name]++
		s.chandyLamportServer.SaveStateToFile()

		s.SendMarkerToOutgoingChannels(s.name)
		// s.chandyLamportServer.RecordIncomingMessage()
	}

	log.Printf("AnotherVerification -> expected value passed from VerifyTransaction of the confidence score: 1 - actial value: %d", req.ConfidenceScore)

	// verify if this node knows the receiver
	if _, known := s.peers[req.TargetNode]; known {
		req.ConfidenceScore++
		log.Printf("%s knows the target node: %s", s.name, req.TargetNode)
	}

	log.Printf("AnotherVerification -> expected value of updated confidence score: 2 - actual value: %d", req.ConfidenceScore)

	// check if exist a successor that it's not the receiver
	if s.successor != "" && s.successor != req.TargetNode {

		// verify if the node must be record its state
		if s.chandyLamportServer.Recording[s.name] {
			// update state
			s.chandyLamportServer.LocalState = "AnotherVerification - " + "transaction verified; actual confidence score: " + strconv.Itoa(int(req.ConfidenceScore))
			s.chandyLamportServer.Version[s.name]++
			s.chandyLamportServer.SaveStateToFile()
		}

		log.Printf("%s calls AnotherVerification on successor: %s", s.name, s.successor)
		peerAddr := s.peers[s.successor]
		conn, err := grpc.Dial(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return &pbNode.TransactionVerificationResponse{ConfidenceScore: req.ConfidenceScore}, err
		}
		defer conn.Close()

		client := pbNode.NewNodeClient(conn)
		resp, err := client.AnotherVerification(ctx, req)
		if err != nil {
			return &pbNode.TransactionVerificationResponse{ConfidenceScore: req.ConfidenceScore}, err
		}
		req.ConfidenceScore = resp.ConfidenceScore
	}

	// this node is the last one before receiver -> send the amount
	if s.successor == req.TargetNode {
		log.Printf("this node is the last one before receiver! Sending amount...")
		time.Sleep(2 * time.Second)
		if req.ConfidenceScore == int32(len(s.peers)-1) {
			log.Printf("AnotherVerification -> expected value of the confidence score before send the amount: 2 - actual value: %d", req.ConfidenceScore)
			finalPeerAddr, ok := s.peers[req.TargetNode]
			if ok {
				conn, err := grpc.DialContext(ctx, finalPeerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					return &pbNode.TransactionVerificationResponse{Known: false, ConfidenceScore: req.ConfidenceScore}, err
				}
				defer conn.Close()

				log.Printf("balance updating...")
				time.Sleep(2 * time.Second)

				client := pbNode.NewNodeClient(conn)
				updateReq := &pbNode.UpdateBalanceRequest{Amount: req.Amount}
				updateResp, err := client.UpdateBalance(ctx, updateReq)
				if err != nil || !updateResp.Success {
					return &pbNode.TransactionVerificationResponse{Known: false, ConfidenceScore: req.ConfidenceScore}, err
				}

				log.Printf("sending the amount...")

				// verify if the node must be record its state
				if s.chandyLamportServer.Recording[s.name] {
					// update state
					s.chandyLamportServer.LocalState = "AnotherVerification - " + "transaction verified; sent " + strconv.Itoa(int(req.Amount)) + "to " + req.TargetNode
					s.chandyLamportServer.Version[s.name]++
					s.chandyLamportServer.SaveStateToFile()
				}

				return &pbNode.TransactionVerificationResponse{Known: true, ConfidenceScore: req.ConfidenceScore}, nil
			}
		} else {
			return &pbNode.TransactionVerificationResponse{Known: false, ConfidenceScore: req.ConfidenceScore}, nil
		}
	}

	return &pbNode.TransactionVerificationResponse{ConfidenceScore: req.ConfidenceScore}, nil
}

func (s *NodeServer) TransferMoney(ctx context.Context, req *pbNode.TransferRequest) (*pbNode.TransferResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("Transfer request received: Sender:%s, Receiver:%s, Amount:%d", req.Sender, req.Receiver, req.Amount)
	log.Printf("Current balance: %d", s.balance)

	// verify if balance is sufficient
	if s.balance < req.Amount {
		log.Printf("Insufficient balance: required=%d, available=%d", req.Amount, s.balance)
		return &pbNode.TransferResponse{Success: false}, nil
	}

	confidenceScore := 0

	// connect to the successor node to verify the transaction
	peerAddr, exists := s.peers[s.successor]
	if !exists || peerAddr == "" {
		log.Printf("Successor node address not found or is empty.")
		return &pbNode.TransferResponse{Success: false}, nil
	}

	log.Printf("Connecting to successor node %s to verify the transaction...", peerAddr)
	conn, err := grpc.Dial(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Failed to connect to successor %s: %v", peerAddr, err)
		return &pbNode.TransferResponse{Success: false}, nil
	}
	defer conn.Close()

	log.Printf("run is here")

	// start Chandy-Lamport snapshot (this is the initiator process)
	if !s.chandyLamportServer.SeenMarkers[s.chandyLamportServer.NodeName] {
		// never seen the marker
		// save local state and version of snapshot
		s.chandyLamportServer.LocalState = "TransferMoney - target node:" + req.Receiver + "; amount:" + strconv.Itoa(int(req.Amount)) + "; current balance: " + strconv.Itoa(int(s.balance))
		// update version of snapshot
		s.chandyLamportServer.Version[s.name]++
		// mark maker as 'seen'
		s.chandyLamportServer.SeenMarkers[s.chandyLamportServer.NodeName] = true
		// record state and send marker on all outgoing channels
		s.chandyLamportServer.SaveStateToFile()
		s.SendMarkerToOutgoingChannels(s.name)
		// start to recording on all incoming channels
		s.chandyLamportServer.Recording[s.name] = true
	}

	client := pbNode.NewNodeClient(conn)
	verificationReq := &pbNode.TransactionVerificationRequest{
		NodeName:        s.name,
		TargetNode:      req.Receiver,
		Amount:          req.Amount,
		ConfidenceScore: int32(confidenceScore),
	}

	resp, err := client.VerifyTransaction(ctx, verificationReq)
	if err != nil {
		log.Printf("Failed to verify transaction with successor %s: %v", peerAddr, err)
		return &pbNode.TransferResponse{Success: false, ConfidenceScore: 0}, nil
	}
	time.Sleep(2000 * time.Millisecond)

	confidenceScore = int(resp.ConfidenceScore)

	/*
		log.Printf("Transaction verified - confidence score: %d/%d", confidenceScore, len(s.peers)-2)

		// no one node know the receiver -> cancel transaction
		if confidenceScore == 0 {
			log.Printf("Receiver unknown... transaction cancelled")
			return &pbNode.TransferResponse{Success: false}, nil
		}
	*/

	finalPeerAddr, ok := s.peers[req.Receiver]
	if !ok || finalPeerAddr == "" {
		return &pbNode.TransferResponse{Success: false}, nil
	}

	conn, err = grpc.DialContext(ctx, finalPeerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return &pbNode.TransferResponse{Success: false}, nil
	}
	defer conn.Close()

	client = pbNode.NewNodeClient(conn)
	updateReq := &pbNode.UpdateBalanceRequest{Amount: req.Amount}
	updateResp, err := client.UpdateBalance(ctx, updateReq)
	if err != nil || !updateResp.Success {
		return &pbNode.TransferResponse{Success: false}, nil
	}
	time.Sleep(2000 * time.Millisecond)

	s.balance -= req.Amount

	// transaction completed successfully and balance updated -> update state of snapshot
	log.Printf("updating state of node 1 to version 2 afeter VerifyTransaction...")
	s.chandyLamportServer.LocalState = "Transaction completed successfully - confidence score: " + strconv.Itoa(confidenceScore)
	s.chandyLamportServer.Version[s.name]++
	s.chandyLamportServer.SaveStateToFile()

	return &pbNode.TransferResponse{Success: true}, nil
}

func (s *NodeServer) RequestApproval(ctx context.Context, req *pbNode.ApprovalRequest) (*pbNode.ApprovalResponse, error) {
	log.Printf("RequestApproval called (Amount: %d, Confidence Score: %d)", req.Amount, req.ConfidenceScore)

	requiredConfidence := len(s.peers) - 2

	// verify if confidence score is sufficient
	if int(req.ConfidenceScore) >= requiredConfidence {
		log.Printf("Transaction approved: Confidence score is sufficient (%d/%d)", req.ConfidenceScore, requiredConfidence)
		return &pbNode.ApprovalResponse{Approved: true}, nil
	}

	log.Printf("Transaction NOT approved by %s: Confidence score is insufficient (%d/%d)", s.name, req.ConfidenceScore, requiredConfidence)
	return &pbNode.ApprovalResponse{Approved: false}, nil
}

func (s *NodeServer) UpdateBalance(ctx context.Context, req *pbNode.UpdateBalanceRequest) (*pbNode.UpdateBalanceResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("updating balance...")
	s.balance += req.Amount

	// verify if the node must be record its state
	if s.chandyLamportServer.SeenMarkerForTheFirstTime[s.name] {
		// record state
		s.chandyLamportServer.LocalState = "UpdateBalance - balance increase by " + strconv.Itoa(int(req.Amount)) + "$"
		s.chandyLamportServer.Version[s.name]++
		s.chandyLamportServer.SaveStateToFile()

		s.SendMarkerToOutgoingChannels(s.name)

	}

	return &pbNode.UpdateBalanceResponse{Success: true}, nil
}

func (s *NodeServer) GetBalance(ctx context.Context, req *pbNode.BalanceRequest) (*pbNode.BalanceResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &pbNode.BalanceResponse{Balance: s.balance}, nil
}

func (s *NodeServer) GetAllNodes(ctx context.Context, req *pbNode.Empty) (*pbRegistry.NodeListResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	nodeList := &pbRegistry.NodeListResponse{}
	for name, address := range s.peers {
		nodeList.Nodes = append(nodeList.Nodes, &pbRegistry.Node{Name: name, Address: address})
	}
	return nodeList, nil
}

func (s *NodeServer) SendMarkerToOutgoingChannels(node string) {
	nodesToSend := []string{}

	// send marker on all outgoing channels
	if s.successor != "" {
		nodesToSend = append(nodesToSend, s.successor)
	}
	if s.predecessor != "" {
		nodesToSend = append(nodesToSend, s.predecessor)
	}

	log.Printf("%s sending marker to outgoing channels: %s %s", s.name, s.successor, s.predecessor)

	for _, peer := range nodesToSend {
		peerAddr, exists := s.peers[peer]
		if exists {

			conn, err := grpc.Dial(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				log.Printf("Failed to connect to peer %s: %v", peer, err)
				continue
			}
			defer conn.Close()

			client := pbNode.NewNodeClient(conn)
			marker := &pbNode.MarkerMessage{
				FromNode: s.name,
			}

			log.Printf("calling ReceiveMarker on: %s", peer)

			_, err = client.ReceiveMarker(context.Background(), marker)
			if err != nil {
				log.Printf("Failed to send marker to peer %s: %v", peer, err)
			} else {
				log.Printf("Marker sent from %s to %s", s.name, peer)
			}
		}
	}
}

/*
func (s *NodeServer) ReceiveMarker(ctx context.Context, msg *pbNode.MarkerMessage) (*pbNode.Empty, error) {
	// Acquire lock only for critical section
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("%s received marker from %s", s.name, msg.FromNode)

	if !s.chandyLamportServer.SeenMarkers[s.name] {
		s.chandyLamportServer.SeenMarkerFrom[s.name] = msg.FromNode
		s.chandyLamportServer.SeenMarkers[s.name] = true
		s.chandyLamportServer.SeenMarkerForTheFirstTime[s.name] = true
		s.chandyLamportServer.Recording[s.name] = true

		log.Printf("%s seen the marker for the first time - start recording", s.name)
	} else {
		log.Printf("%s has already seen a marker: stop recording on that channel", s.name)
		s.chandyLamportServer.StopRecordingOnChannel(s.name)
	}

	return &pbNode.Empty{}, nil
}

*/

func (s *NodeServer) ReceiveMarker(ctx context.Context, msg *pbNode.MarkerMessage) (*pbNode.Empty, error) {
	go func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		time.Sleep(1 * time.Second)
		log.Printf("%s received marker from %s", s.name, msg.FromNode)

		// node never seen marker yet
		if !s.chandyLamportServer.SeenMarkers[s.name] {
			// received marker form node 'msg.FromNode'
			s.chandyLamportServer.SeenMarkerFrom[s.name] = msg.FromNode
			// mark marker as 'seen'
			s.chandyLamportServer.SeenMarkers[s.name] = true
			s.chandyLamportServer.SeenMarkerForTheFirstTime[s.name] = true // seen marker one time
			// start recording
			s.chandyLamportServer.Recording[s.name] = true
			log.Printf("%s seen the marker for the first time - start recording its state and incoming message", s.name)
		} else {
			log.Printf("%s has already seen a marker: stop recording on that channel", s.name)
			log.Printf("calling StopRecordingOnChannel on %s", s.name)
			s.chandyLamportServer.StopRecordingOnChannel(s.name)
		}
	}()

	return &pbNode.Empty{}, nil
}

func (s *NodeServer) UpdatePeers(ctx context.Context, req *pbNode.NodeListResponse) (*pbNode.Empty, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, node := range req.Nodes {
		s.peers[node.Name] = node.Address
	}

	var nodeNames []string
	for name := range s.peers {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	// found successor and predecessor
	for i, nodeName := range nodeNames {
		if nodeName == s.name {
			if i > 0 {
				s.predecessor = nodeNames[i-1]
				log.Printf("now the predecessor is %s", s.predecessor)
			} else {
				s.predecessor = ""
				log.Printf("no predecessor")
			}

			if i < len(nodeNames)-1 {
				s.successor = nodeNames[i+1]
				log.Printf("now the successor is %s", s.successor)
			} else {
				s.successor = ""
				log.Printf("no successor")
			}
			break
		}
	}

	log.Printf("Peers updated: %v", s.peers)
	return &pbNode.Empty{}, nil
}

func (s *NodeServer) getPeerNameByAddress(address string) string {
	for name, addr := range s.peers {
		if addr == address {
			return name
		}
	}
	return ""
}

func StartNodeServer(config NodeConfig) {
	time.Sleep(5 * time.Second)
	cls := chandyLamport.NewChandyLamportServer(config.Name)

	lis, err := net.Listen("tcp", config.Address)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	balance, err := strconv.Atoi(config.Balance)
	if err != nil {
		log.Fatalf("Invalid initial balance: %v", err)
	}

	// call interceptor
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(cls.UnaryServerInterceptor()),
	)

	// creating node server
	nodeServer := NewNodeServer(config.Name, int32(balance), config.RegistryAddress, cls)

	// register node server
	pbNode.RegisterNodeServer(grpcServer, nodeServer)
	reflection.Register(grpcServer)

	log.Printf("Node server %s listening on %s", config.Name, config.Address)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
		}
	}()

	conn, err := grpc.Dial(configNode.ServerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithTimeout(10*time.Second))
	if err != nil {
		log.Fatalf("Failed to connect to registry: %v", err)
	}

	defer conn.Close()

	registryClient := pbRegistry.NewRegistryClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req := &pbRegistry.RegisterRequest{Name: config.Name, Address: config.Address}
	_, err = registryClient.RegisterNode(ctx, req)
	if err != nil {
		log.Fatalf("Failed to register node: %v", err)
	}
	log.Printf("Node %s registered with registry at %s", config.Name, config.RegistryAddress)

	/* periodically update the peers list
	go func() {
		for {
			nodeServer.GetPeers()
			time.Sleep(5 * time.Second)
		}
	}()
	*/

}
