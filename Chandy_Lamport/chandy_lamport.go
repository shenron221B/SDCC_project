package Chandy_Lamport

import (
	pbNode "SDCC/node/registry"
	"fmt"
	"google.golang.org/grpc"
	"log"
	"os"
	"sync"
)

import "context"

type ChandyLamportServer struct {
	NodeName                  string
	Version                   map[string]int
	LocalState                string
	SeenMarkers               map[string]bool
	Recording                 map[string]bool
	SeenMarkerForTheFirstTime map[string]bool
	SeenMarkerFrom            map[string]string
	ChannelState              map[string]string
	MessageLogs               map[string][]interface{}
	mutex                     sync.Mutex
}

func NewChandyLamportServer(nodeName string) *ChandyLamportServer {
	return &ChandyLamportServer{
		NodeName:                  nodeName,
		SeenMarkers:               make(map[string]bool),   // if a node has seen a marker
		SeenMarkerFrom:            make(map[string]string), // from who a node has received the marker
		Version:                   make(map[string]int),    // version of snapshot
		Recording:                 make(map[string]bool),   // if a node is actual recording
		SeenMarkerForTheFirstTime: make(map[string]bool),   // if the marker is seen for the first time
		MessageLogs:               make(map[string][]interface{}),
		ChannelState:              make(map[string]string), // state of channels
	}

}

func (cls *ChandyLamportServer) StopRecordingOnChannel(node string) {
	cls.mutex.Lock()
	defer cls.mutex.Unlock()

	if cls.Recording[node] {
		cls.Recording[node] = false
		log.Printf("%s stop recording incoming message", node)
	}
}

func (cls *ChandyLamportServer) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {

		// extract name of RPC call
		operationName := extractOperationName(req)
		if operationName == "UnknownOperation" {
			return handler(ctx, req) // continue with the normal request processing
		}

		log.Printf("Intercepted RPC call: %s on node %s", operationName, cls.NodeName)

		// verify if node is recording
		if cls.Recording[cls.NodeName] {
			// save state of the incoming channel
			cls.ChannelState[cls.NodeName] = operationName
			cls.RecordChannelState(cls.NodeName, operationName)
		} else {
			log.Printf("%s is not recording, no state will be saved", cls.NodeName)
		}

		// continue with the normal request processing
		return handler(ctx, req)
	}
}

func extractOperationName(req interface{}) string {
	switch req.(type) {
	case *pbNode.TransactionVerificationRequest:
		return "VerifyTransaction"
	case *pbNode.TransferRequest:
		return "TransferMoney"
	case *pbNode.UpdateBalanceRequest:
		return "UpdateBalance"
	case *pbNode.TransactionVerificationResponse:
		return "VerifyTransactionResponse"
	case *pbNode.ApprovalRequest:
		return "ApprovalRequest"
	default:
		return "UnknownOperation"
	}
}

func (cls *ChandyLamportServer) RecordChannelState(node, operation string) {
	state := fmt.Sprintf("channel C[%s]\nstate: %s", node, operation)
	cls.ChannelState[node] = state

	filename := "distributed_snapshot.txt"
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	file.WriteString(fmt.Sprintf("%s\n\n", state))
	log.Printf("Recorded channel state to file: %s", state)
}

func (cls *ChandyLamportServer) SaveStateToFile() {
	filename := "distributed_snapshot.txt"
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	file.WriteString(fmt.Sprintf("Node: %s\n", cls.NodeName))
	file.WriteString(fmt.Sprintf("State: %s\n", cls.LocalState))
	file.WriteString(fmt.Sprintf("Version: %d\n", cls.Version[cls.NodeName]))
	file.WriteString(fmt.Sprintf("\n"))

	log.Printf("State saved to %s", filename)
}
