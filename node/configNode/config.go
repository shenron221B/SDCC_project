package configNode

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

var (
	ServerAddress string
	MyAddress     string
)

// ServiceRegistry struct for service registry
type ServiceRegistry struct {
	Address string `json:"address"`
	Port    string `json:"port"`
}

// Config struct for configuration of node in JSON file
type Config struct {
	Localhost LocalhostConfig `json:"localhost"`
	Docker    DockerConfig    `json:"docker"`
}

// LocalhostConfig struct for localhost configuration
type LocalhostConfig struct {
	ServiceRegistry ServiceRegistry `json:"service_registry"`
}

// DockerConfig struct for Docker configuration
type DockerConfig struct {
	ServiceRegistry ServiceRegistry `json:"service_registry"`
	Peers           []PeerConfig    `json:"peers"`
}

// PeerConfig specific configuration for a peer
type PeerConfig struct {
	Address string `json:"address"`
	Port    string `json:"port"`
	Balance string `json:"balance"`
}

func DockerConfiguration() (string, string, string) {
	// read config.json
	fileContent, err := os.ReadFile("/config.json")
	if err != nil {
		fmt.Println("read file error:", err)
		return "", "", ""
	}

	// parsing JSON
	var configData Config
	err = json.Unmarshal(fileContent, &configData)
	if err != nil {
		fmt.Println("error during parsing of JSON file:", err)
		return "", "", ""
	}

	// setting registry address
	ServerAddress = configData.Docker.ServiceRegistry.Address + configData.Docker.ServiceRegistry.Port

	// get peer name from environment variable
	peerName := os.Getenv("PEER_NAME")
	if peerName == "" {
		fmt.Println("PEER_NAME not found")
		return "", "", ""
	}

	fmt.Printf("Peer rilevato: %s\n", peerName)

	// check PEER_NAME in configuration file
	for _, peer := range configData.Docker.Peers {
		if peer.Address == peerName {
			MyAddress = peer.Address + peer.Port
			return MyAddress, peer.Address, peer.Balance
		}
	}

	fmt.Println("peer not found in configuration file")
	return "", "", ""
}

func LocalConfig() {
	fileContent, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("file read error: %v", err)
	}

	var configData Config
	err = json.Unmarshal(fileContent, &configData)
	if err != nil {
		log.Fatalf("error during parsing JSON file: %v", err)
	}

	// setting registry address
	ServerAddress = configData.Localhost.ServiceRegistry.Address + configData.Localhost.ServiceRegistry.Port

	MyAddress = os.Args[6]
	fmt.Println("localhost configuration complete - node address: " + MyAddress)
}
