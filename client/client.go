package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
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

// This function is used only for Docker running to wait that all nodes are registered on service registry
func getAllNodesWithRetry(registryAddr string) {
	// Connect to the registry server
	log.Println("Connecting to registry server...")
	conn, err := grpc.Dial(registryAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to registry: %v", err)
	}
	defer conn.Close()

	registryClient := pbRegistry.NewRegistryClient(conn)

	// Retry loop to wait until all nodes are registered
	for {
		// Set a timeout for the context
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Call GetAllNodes RPC
		resp, err := registryClient.GetAllNodes(ctx, &pbRegistry.Empty{})
		if err != nil {
			log.Printf("Error retrieving nodes: %v", err)
		} else if len(resp.Nodes) >= 3 {
			// Exit loop once we have some nodes registered
			log.Println("Successfully retrieved nodes from registry")
			break
		}

		// If no nodes are returned or there's an error, retry after a short delay
		log.Println("No nodes retrieved yet, retrying...")
		time.Sleep(2 * time.Second)
	}

	return
}

func main() {
	localhostFlag := flag.Bool("localhost", false, "if program run on localhost")
	dockerFlag := flag.Bool("docker", false, "if program run on Docker")
	flag.Parse()

	registryAddr := ""
	peerAddr := ""

	var sender, receiver string
	var amount int
	var err error

	if *dockerFlag {
		log.Printf("Docker environment")

		// read env variables
		sender = os.Getenv("SENDER")
		receiver = os.Getenv("RECEIVER")
		amountStr := os.Getenv("AMOUNT")

		if sender == "" || receiver == "" || amountStr == "" {
			log.Fatalf("environment variables SENDER, RECEIVER and AMOUNT must be setting")
		}

		amount, err = strconv.Atoi(amountStr)
		if err != nil {
			log.Fatalf("amount not correct: %v", err)
		}

		registryAddr = "service_registry:50051"
		peerAddr = "peer-1:50052"

		// wait that all nodes are registered to registry
		getAllNodesWithRetry(registryAddr)
	} else if *localhostFlag {
		log.Printf("running on localhost")

		if len(os.Args) != 5 {
			log.Fatalf("usage: client -localhost sender receiver amount")
		}

		sender = os.Args[2]
		receiver = os.Args[3]
		amount, err = strconv.Atoi(os.Args[4])
		if err != nil {
			log.Fatalf("amount not correct: %v", err)
		}

		registryAddr = ":50051"
		peerAddr = ":50052"
	} else {
		log.Fatalf("mode error. Usage -docker or -localhost")
	}

	// continue with client logic
	printAllNodeBalances(registryAddr)

	// connection to the sender node
	conn, err := grpc.Dial(peerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	client := pbNode.NewNodeClient(conn)

	req := &pbNode.TransferRequest{
		Sender:   sender,
		Receiver: receiver,
		Amount:   int32(amount),
	}

	log.Printf("Sending request from %s to %s", req.Sender, req.Receiver)
	resp, err := client.TransferMoney(context.Background(), req)
	if err != nil {
		log.Fatalf("Error during transfer: %v", err)
	}

	if resp.Success {
		log.Println("Amount successfully transferred")
	} else {
		log.Println("Transaction failed")
	}

	// print balance again to verify the transaction
	printAllNodeBalances(registryAddr)
}
