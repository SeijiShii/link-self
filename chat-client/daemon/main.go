package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/SeijiShii/link-self/core/internal/did"
	"github.com/SeijiShii/link-self/core/internal/node"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
	ID      interface{} `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
	ID      interface{} `json:"id"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type JSONRPCNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type StartParams struct {
	ListenAddrs    []string `json:"listenAddrs,omitempty"`
	BootstrapPeers  []string `json:"bootstrapPeers,omitempty"`
	IdentityPath   string   `json:"identityPath,omitempty"`
}

type StartResult struct {
	DID       string `json:"did"`
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
	linkSelfNode *node.Node
	ctx          context.Context
	cancel       context.CancelFunc
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

	// Load or generate identity
	identityPath := params.IdentityPath
	if identityPath == "" {
		homeDir, _ := os.UserHomeDir()
		identityPath = filepath.Join(homeDir, ".linkself", "identity.json")
	}

	identity, err := loadOrGenerateIdentity(identityPath)
	if err != nil {
		sendError(req.ID, -32000, "Failed to load identity", err.Error())
		return
	}

	// Parse bootstrap peers
	var bootstrapPeers []peer.AddrInfo
	for _, addrStr := range params.BootstrapPeers {
		info, err := peer.AddrInfoFromString(addrStr)
		if err != nil {
			sendError(req.ID, -32602, "Invalid bootstrap peer", err.Error())
			return
		}
		bootstrapPeers = append(bootstrapPeers, *info)
	}

	// Set default listen address if not provided
	listenAddrs := params.ListenAddrs
	if len(listenAddrs) == 0 {
		listenAddrs = []string{"/ip4/127.0.0.1/tcp/0"}
	}

	// Create node
	cfg := node.Config{
		Identity:       identity,
		ListenAddrs:    listenAddrs,
		BootstrapPeers: bootstrapPeers,
	}

	n, err := node.New(ctx, cfg)
	if err != nil {
		sendError(req.ID, -32000, "Failed to create node", err.Error())
		return
	}

	// Set message handler
	n.SetOnMessage(func(peerDID string, payload []byte) {
		sendNotification("onMessage", MessageNotificationParams{
			PeerDID: peerDID,
			Payload: string(payload),
		})
	})

	// Start node
	if err := n.Start(ctx); err != nil {
		sendError(req.ID, -32000, "Failed to start node", err.Error())
		return
	}

	linkSelfNode = n

	// Get listen address
	listenAddr := ""
	if len(n.Host.Addrs()) > 0 {
		listenAddr = fmt.Sprintf("%s/p2p/%s", n.Host.Addrs()[0].String(), n.Host.ID().String())
	}

	result := StartResult{
		DID:        identity.DID,
		ListenAddr: listenAddr,
	}

	sendResponse(req.ID, result)
}

func handleStop(req *JSONRPCRequest) {
	if linkSelfNode != nil {
		linkSelfNode.Close()
		linkSelfNode = nil
	}
	cancel()
	sendResponse(req.ID, nil)
}

func handleGetMyDID(req *JSONRPCRequest) {
	if linkSelfNode == nil {
		sendError(req.ID, -32000, "Node not started", "Node must be started first")
		return
	}
	sendResponse(req.ID, linkSelfNode.Identity.DID)
}

func handleSendMessage(req *JSONRPCRequest) {
	if linkSelfNode == nil {
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

	// Send message (1-to-1 is a 2-person group)
	memberDIDs := []string{linkSelfNode.Identity.DID, params.PeerDID}
	err := linkSelfNode.SendToGroup(ctx, memberDIDs, []byte(params.Message))
	if err != nil {
		sendError(req.ID, -32000, "Failed to send message", err.Error())
		return
	}

	sendResponse(req.ID, nil)
}

func handleConnect(req *JSONRPCRequest) {
	if linkSelfNode == nil {
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

	_, err := linkSelfNode.Connect(ctx, params.PeerDID)
	if err != nil {
		sendError(req.ID, -32000, "Failed to connect", err.Error())
		return
	}

	sendResponse(req.ID, nil)
}

func loadOrGenerateIdentity(path string) (*did.Identity, error) {
	// Try to load existing identity
	if data, err := os.ReadFile(path); err == nil {
		var keyData struct {
			PrivKey []byte `json:"privKey"`
		}
		if err := json.Unmarshal(data, &keyData); err == nil {
			privKey, err := crypto.UnmarshalPrivateKey(keyData.PrivKey)
			if err == nil {
				return did.FromPrivKey(privKey)
			}
		}
	}

	// Generate new identity
	identity, err := did.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate identity: %w", err)
	}

	// Save identity
	if err := saveIdentity(path, identity); err != nil {
		// Log error but don't fail
		fmt.Fprintf(os.Stderr, "Warning: failed to save identity: %v\n", err)
	}

	return identity, nil
}

func saveIdentity(path string, identity *did.Identity) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Marshal private key
	privKeyBytes, err := crypto.MarshalPrivateKey(identity.PrivKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	keyData := struct {
		PrivKey []byte `json:"privKey"`
	}{
		PrivKey: privKeyBytes,
	}

	data, err := json.Marshal(keyData)
	if err != nil {
		return fmt.Errorf("failed to marshal identity: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write identity file: %w", err)
	}

	return nil
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
