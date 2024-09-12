package main

import (
	"SDCC/node"
	"SDCC/node/configNode"
	pbNode "SDCC/node/registry"
	"SDCC/server_registry"
	"flag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"os"
	"os/signal"
	"syscall"
)

import "context"

func main() {
	log.Printf("Arguments: %v", os.Args)

	// flags to control the environment
	localhostFlag := flag.Bool("localhost", false, "if program run on localhost")
	dockerFlag := flag.Bool("docker", false, "if program run on Docker")
	flag.Parse()

	// check run mode (registry or node)
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <registry|node>", os.Args[0])
	}
	mode := os.Args[2]

	switch mode {
	case "registry":
		// run service registry
		log.Printf("registry mode")
		server_registry.StartRegistryServer(":50051")

	case "node":
		if *localhostFlag {
			// run node - check parameters
			if len(os.Args) < 6 {
				log.Fatalf("Usage: %s node <name> <balance> <registry address> <node port>", os.Args[0])
			}
			configNode.LocalConfig()

			// node configuration
			config := node.NodeConfig{
				Address:         configNode.MyAddress,
				Name:            os.Args[3],
				Balance:         os.Args[4],
				RegistryAddress: configNode.ServerAddress,
			}

			node.StartNodeServer(config)
		} else if *dockerFlag {
			address, name, balance := configNode.DockerConfiguration()
			if address == "" || name == "" || balance == "" {
				log.Fatal("Docker configuration failed")
			}

			// node configuration
			config := node.NodeConfig{
				Address:         address,
				Name:            name,
				Balance:         balance,
				RegistryAddress: configNode.ServerAddress,
			}

			// start node
			node.StartNodeServer(config)

			RunClientOnDocker()
		}
	default:
		log.Fatalf("unknown mode: %s", mode)
	}

	// wait for interrupt signal to gracefully shut down the server
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Println("Shutting down")
}

func RunClientOnDocker() {
	sender := "peer-1"
	receiver := "peer-3"
	amount := 500

	// registry address
	// registryAddr := ":50051"

	// connecting to node server
	conn, err := grpc.Dial(":50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pbNode.NewNodeClient(conn)

	// setting transaction request
	req := &pbNode.TransferRequest{
		Sender:   sender,
		Receiver: receiver,
		Amount:   int32(amount),
	}

	// sending transaction
	log.Printf("sending transfer request from %s to %s", req.Sender, req.Receiver)
	resp, err := client.TransferMoney(context.Background(), req)
	if err != nil {
		log.Fatalf("Error during transfer: %v", err)
	}

	if resp.Success {
		log.Println("amount successfully transfer")
	} else {
		log.Println("transaction failed")
	}
}
