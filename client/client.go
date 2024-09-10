package main

import (
	"context"
	"fmt"
	"log"
	"time"

	pbNode "SDCC/node/registry"
	pbRegistry "SDCC/server_registry/registry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Function to get balance of a node
func getBalance(client pbNode.NodeClient, nodeName string) (int32, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req := &pbNode.BalanceRequest{Name: nodeName}
	resp, err := client.GetBalance(ctx, req)
	if err != nil {
		return 0, err
	}
	return resp.Balance, nil
}

// Function to get all nodes and print their balances
func printAllNodeBalances(registryAddr string) {
	// Connect to the registry server
	log.Println("Connecting to registry server...")
	conn, err := grpc.Dial(registryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to registry: %v", err)
	}
	defer conn.Close()

	registryClient := pbRegistry.NewRegistryClient(conn)

	// Get all nodes
	log.Println("Getting all nodes...")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	req := &pbRegistry.Empty{}
	resp, err := registryClient.GetAllNodes(ctx, req)
	if err != nil {
		log.Fatalf("Failed to get all nodes: %v", err)
	}

	log.Println("Retrieved nodes from registry.")

	// For each node, get its balance
	for _, node := range resp.Nodes {
		log.Printf("Connecting to node %s at %s...", node.Name, node.Address)
		conn, err := grpc.Dial(node.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Failed to connect to node %s: %v", node.Name, err)
			continue
		}
		defer conn.Close()

		client := pbNode.NewNodeClient(conn)
		balance, err := getBalance(client, node.Name)
		if err != nil {
			log.Printf("Failed to get balance for node %s: %v", node.Name, err)
			continue
		}

		fmt.Printf("Node %s (Address: %s) Balance: %d\n", node.Name, node.Address, balance)
	}
}

func main() {
	registryAddr := ":50051" // address of server registry
	printAllNodeBalances(registryAddr)
	// connection to node
	conn, err := grpc.Dial(":50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	client := pbNode.NewNodeClient(conn)

	// create transfer request
	req := &pbNode.TransferRequest{
		Sender:   "peer-1",
		Receiver: "peer-4",
		Amount:   500,
	}

	// send request
	log.Printf("Sending transfer request from %s to %s", req.Sender, req.Receiver)
	resp, err := client.TransferMoney(context.Background(), req)
	if err != nil {
		log.Fatalf("Error during transfer: %v", err)
	}

	if resp.Success {
		log.Println("Transfer successful")
	} else {
		log.Println("Transfer failed")
	}
	printAllNodeBalances(registryAddr)
}
