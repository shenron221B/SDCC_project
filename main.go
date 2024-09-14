package main

import (
	"SDCC/node"
	"SDCC/node/configNode"
	"SDCC/server_registry"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

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
			log.Printf("Docker environment")
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
