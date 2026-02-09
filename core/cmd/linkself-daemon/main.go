package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SeijiShii/link-self/core/pkg/linkself"
)

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	Result  interface{}  `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
	ID      interface{}   `json:"id"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type StartParams struct {
	ListenAddrs    []string `json:"listenAddrs,omitempty"`
	BootstrapPeers []string `json:"bootstrapPeers,omitempty"`
	IdentityPath   string   `json:"identityPath,omitempty"`
	UsePublicDHT   bool     `json:"usePublicDHT,omitempty"`
}

type StartResult struct {
	DID        string `json:"did"`
	ListenAddr string `json:"listenAddr"`
}

type SendMessageParams struct {
	PeerDID string `json:"peerDID"`
	Message string `json:"message"`
}

type ConnectParams struct {
	PeerDID string `json:"peerDID"`
}

type MessageNotificationParams struct {
	PeerDID string `json:"peerDID"`
	Payload string `json:"payload"`
}

var (
	linkSelfClient linkself.Client
	ctx            context.Context
	cancel         context.CancelFunc
)

func main() {
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			sendError(nil, -32700, "Parse error", err.Error())
			continue
		}

		if req.JSONRPC != "2.0" {
			sendError(req.ID, -32600, "Invalid Request", "jsonrpc must be 2.0")
			continue
		}

		handleRequest(&req)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
	}
}

func handleRequest(req *JSONRPCRequest) {
	switch req.Method {
	case "start":
		handleStart(req)
	case "stop":
		handleStop(req)
	case "getMyDID":
		handleGetMyDID(req)
	case "sendMessage":
		handleSendMessage(req)
	case "connect":
		handleConnect(req)
	default:
		sendError(req.ID, -32601, "Method not found", fmt.Sprintf("Unknown method: %s", req.Method))
	}
}

func handleStart(req *JSONRPCRequest) {
	var params StartParams
	if req.Params != nil {
		paramsBytes, err := json.Marshal(req.Params)
		if err != nil {
			sendError(req.ID, -32602, "Invalid params", err.Error())
			return
		}
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			sendError(req.ID, -32602, "Invalid params", err.Error())
			return
		}
	}

	// Create client using public API
	client := linkself.NewClient()

	// Configure using public API
	config := linkself.Config{
		IdentityPath:   params.IdentityPath,
		ListenAddrs:    params.ListenAddrs,
		BootstrapPeers: params.BootstrapPeers,
		UsePublicDHT:   params.UsePublicDHT,
	}

	// Start node using public API
	info, err := client.Start(ctx, config)
	if err != nil {
		sendError(req.ID, -32000, "Failed to start", err.Error())
		return
	}

	// Set message handler
	client.SetOnMessage(func(peerDID string, payload []byte) {
		sendNotification("onMessage", MessageNotificationParams{
			PeerDID: peerDID,
			Payload: string(payload),
		})
	})

	linkSelfClient = client

	result := StartResult{
		DID:        info.DID,
		ListenAddr: info.ListenAddr,
	}

	sendResponse(req.ID, result)
}

func handleStop(req *JSONRPCRequest) {
	if linkSelfClient != nil {
		linkSelfClient.Stop(ctx)
		linkSelfClient = nil
	}
	cancel()
	sendResponse(req.ID, nil)
}

func handleGetMyDID(req *JSONRPCRequest) {
	if linkSelfClient == nil {
		sendError(req.ID, -32000, "Node not started", "Node must be started first")
		return
	}
	sendResponse(req.ID, linkSelfClient.GetMyDID())
}

func handleSendMessage(req *JSONRPCRequest) {
	if linkSelfClient == nil {
		sendError(req.ID, -32000, "Node not started", "Node must be started first")
		return
	}

	var params SendMessageParams
	if req.Params != nil {
		paramsBytes, err := json.Marshal(req.Params)
		if err != nil {
			sendError(req.ID, -32602, "Invalid params", err.Error())
			return
		}
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			sendError(req.ID, -32602, "Invalid params", err.Error())
			return
		}
	}

	err := linkSelfClient.SendMessage(ctx, params.PeerDID, params.Message)
	if err != nil {
		sendError(req.ID, -32000, "Failed to send message", err.Error())
		return
	}

	sendResponse(req.ID, nil)
}

func handleConnect(req *JSONRPCRequest) {
	if linkSelfClient == nil {
		sendError(req.ID, -32000, "Node not started", "Node must be started first")
		return
	}

	var params ConnectParams
	if req.Params != nil {
		paramsBytes, err := json.Marshal(req.Params)
		if err != nil {
			sendError(req.ID, -32602, "Invalid params", err.Error())
			return
		}
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			sendError(req.ID, -32602, "Invalid params", err.Error())
			return
		}
	}

	err := linkSelfClient.Connect(ctx, params.PeerDID)
	if err != nil {
		sendError(req.ID, -32000, "Failed to connect", err.Error())
		return
	}

	sendResponse(req.ID, nil)
}

func sendResponse(id interface{}, result interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      id,
	}
	data, _ := json.Marshal(resp)
	fmt.Println(string(data))
}

func sendError(id interface{}, code int, message string, data interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
		ID: id,
	}
	jsonData, _ := json.Marshal(resp)
	fmt.Println(string(jsonData))
}

func sendNotification(method string, params interface{}) {
	notif := JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	data, _ := json.Marshal(notif)
	fmt.Println(string(data))
}
