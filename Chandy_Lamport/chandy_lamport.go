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
	IncomingMessageFrom       map[string]string
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
		IncomingMessageFrom:       make(map[string]string), // to keep trace of messages sent on channels
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

		// get the method name from the incoming RPC info
		methodName := info.FullMethod
		log.Printf("Intercepted RPC call: %s", methodName)

		// check if this node have incoming messages
		incomingMessage, exists := cls.IncomingMessageFrom[cls.NodeName]
		if !exists {
			log.Printf("No incoming message found for node %s", cls.NodeName)
			return handler(ctx, req) // continue with the normal request processing
		}

		peerNode := incomingMessage[:6] // extract peer name that has sent the incoming message
		log.Printf("incoming message from %s", peerNode)
		rpcCall := incomingMessage[6:] // extract RPC call
		log.Printf("rpcCall: %s", rpcCall)
		if cls.Recording[cls.NodeName] {
			log.Printf("%s is recording", cls.NodeName)
		}

		if methodName == rpcCall {
			// verify if that node is recording
			if cls.Recording[peerNode] {
				log.Printf("Recording channel state for %s from %s", peerNode, cls.NodeName)
				// save the state of channel
				cls.RecordChannelState(cls.NodeName, peerNode, methodName)
			} else {
				log.Printf("%s is not recording", peerNode)
			}
		} else {
			log.Printf("Method %s does not match recorded RPC call %s", methodName, rpcCall)
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
	default:
		return "UnknownOperation"
	}
}

func (cls *ChandyLamportServer) RecordChannelState(fromNode, toNode, operation string) {
	state := fmt.Sprintf("channel C[%s:%s]\nstate: %s", fromNode, toNode, operation)
	cls.ChannelState[toNode] = state

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
