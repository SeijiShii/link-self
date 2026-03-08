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
	PeerDID   string `json:"peerDID"`
	ListenAddr string `json:"listenAddr,omitempty"`
}

type MessageNotificationParams struct {
	PeerDID string `json:"peerDID"`
	Payload string `json:"payload"`
}

// DeviceDB params
type DeviceDBPutParams struct {
	Table    string          `json:"table"`
	RecordID string          `json:"recordID"`
	Body     json.RawMessage `json:"body"`
}

type DeviceDBGetParams struct {
	Table    string `json:"table"`
	RecordID string `json:"recordID"`
}

type DeviceDBDeleteParams struct {
	Table    string `json:"table"`
	RecordID string `json:"recordID"`
}

type DeviceDBListParams struct {
	Table string `json:"table"`
}

// GroupShare params
type GroupShareRegisterParams struct {
	Channel string `json:"channel"`
	GroupID string `json:"groupID"`
}

type GroupSharePutParams struct {
	Channel  string          `json:"channel"`
	RecordID string          `json:"recordID"`
	Body     json.RawMessage `json:"body"`
}

type GroupShareGetParams struct {
	Channel  string `json:"channel"`
	RecordID string `json:"recordID"`
}

type GroupShareDeleteParams struct {
	Channel  string `json:"channel"`
	RecordID string `json:"recordID"`
}

type GroupShareListParams struct {
	Channel string `json:"channel"`
}

// Groups params
type GroupsCreateParams struct {
	MemberDIDs []string `json:"memberDIDs"`
}

type GroupsAddMemberParams struct {
	GroupID   string `json:"groupID"`
	MemberDID string `json:"memberDID"`
}

type GroupsLeaveParams struct {
	GroupID string `json:"groupID"`
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
	case "devicedb.put":
		handleDeviceDBPut(req)
	case "devicedb.get":
		handleDeviceDBGet(req)
	case "devicedb.delete":
		handleDeviceDBDelete(req)
	case "devicedb.list":
		handleDeviceDBList(req)
	case "groupshare.register":
		handleGroupShareRegister(req)
	case "groupshare.put":
		handleGroupSharePut(req)
	case "groupshare.get":
		handleGroupShareGet(req)
	case "groupshare.delete":
		handleGroupShareDelete(req)
	case "groupshare.list":
		handleGroupShareList(req)
	case "groups.create":
		handleGroupsCreate(req)
	case "groups.addMember":
		handleGroupsAddMember(req)
	case "groups.leave":
		handleGroupsLeave(req)
	case "groups.list":
		handleGroupsList(req)
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

	err := linkSelfClient.Connect(ctx, params.PeerDID, params.ListenAddr)
	if err != nil {
		sendError(req.ID, -32000, "Failed to connect", err.Error())
		return
	}

	sendResponse(req.ID, nil)
}

// parseParams is a helper to unmarshal JSON-RPC params into a typed struct.
func parseParams(req *JSONRPCRequest, out interface{}) bool {
	if req.Params == nil {
		sendError(req.ID, -32602, "Invalid params", "params required")
		return false
	}
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return false
	}
	if err := json.Unmarshal(paramsBytes, out); err != nil {
		sendError(req.ID, -32602, "Invalid params", err.Error())
		return false
	}
	return true
}

func requireClient(req *JSONRPCRequest) bool {
	if linkSelfClient == nil {
		sendError(req.ID, -32000, "Node not started", "Node must be started first")
		return false
	}
	return true
}

// --- DeviceDB handlers ---

func handleDeviceDBPut(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params DeviceDBPutParams
	if !parseParams(req, &params) {
		return
	}
	if err := linkSelfClient.DeviceDB().Put(ctx, params.Table, params.RecordID, params.Body); err != nil {
		sendError(req.ID, -32000, "devicedb.put failed", err.Error())
		return
	}
	sendResponse(req.ID, nil)
}

func handleDeviceDBGet(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params DeviceDBGetParams
	if !parseParams(req, &params) {
		return
	}
	rec, err := linkSelfClient.DeviceDB().Get(ctx, params.Table, params.RecordID)
	if err != nil {
		sendError(req.ID, -32000, "devicedb.get failed", err.Error())
		return
	}
	sendResponse(req.ID, rec)
}

func handleDeviceDBDelete(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params DeviceDBDeleteParams
	if !parseParams(req, &params) {
		return
	}
	if err := linkSelfClient.DeviceDB().Delete(ctx, params.Table, params.RecordID); err != nil {
		sendError(req.ID, -32000, "devicedb.delete failed", err.Error())
		return
	}
	sendResponse(req.ID, nil)
}

func handleDeviceDBList(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params DeviceDBListParams
	if !parseParams(req, &params) {
		return
	}
	recs, err := linkSelfClient.DeviceDB().List(ctx, params.Table)
	if err != nil {
		sendError(req.ID, -32000, "devicedb.list failed", err.Error())
		return
	}
	sendResponse(req.ID, recs)
}

// --- GroupShare handlers ---

func handleGroupShareRegister(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params GroupShareRegisterParams
	if !parseParams(req, &params) {
		return
	}
	if err := linkSelfClient.GroupShare().RegisterChannel(params.Channel, params.GroupID); err != nil {
		sendError(req.ID, -32000, "groupshare.register failed", err.Error())
		return
	}
	sendResponse(req.ID, nil)
}

func handleGroupSharePut(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params GroupSharePutParams
	if !parseParams(req, &params) {
		return
	}
	if err := linkSelfClient.GroupShare().Put(ctx, params.Channel, params.RecordID, params.Body); err != nil {
		sendError(req.ID, -32000, "groupshare.put failed", err.Error())
		return
	}
	sendResponse(req.ID, nil)
}

func handleGroupShareGet(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params GroupShareGetParams
	if !parseParams(req, &params) {
		return
	}
	rec, err := linkSelfClient.GroupShare().Get(ctx, params.Channel, params.RecordID)
	if err != nil {
		sendError(req.ID, -32000, "groupshare.get failed", err.Error())
		return
	}
	sendResponse(req.ID, rec)
}

func handleGroupShareDelete(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params GroupShareDeleteParams
	if !parseParams(req, &params) {
		return
	}
	if err := linkSelfClient.GroupShare().Delete(ctx, params.Channel, params.RecordID); err != nil {
		sendError(req.ID, -32000, "groupshare.delete failed", err.Error())
		return
	}
	sendResponse(req.ID, nil)
}

func handleGroupShareList(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params GroupShareListParams
	if !parseParams(req, &params) {
		return
	}
	recs, err := linkSelfClient.GroupShare().List(ctx, params.Channel)
	if err != nil {
		sendError(req.ID, -32000, "groupshare.list failed", err.Error())
		return
	}
	sendResponse(req.ID, recs)
}

// --- Groups handlers ---

func handleGroupsCreate(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params GroupsCreateParams
	if !parseParams(req, &params) {
		return
	}
	groupID, err := linkSelfClient.Groups().CreateGroup(ctx, params.MemberDIDs)
	if err != nil {
		sendError(req.ID, -32000, "groups.create failed", err.Error())
		return
	}
	sendResponse(req.ID, map[string]string{"groupID": groupID})
}

func handleGroupsAddMember(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params GroupsAddMemberParams
	if !parseParams(req, &params) {
		return
	}
	if err := linkSelfClient.Groups().AddMember(ctx, params.GroupID, params.MemberDID); err != nil {
		sendError(req.ID, -32000, "groups.addMember failed", err.Error())
		return
	}
	sendResponse(req.ID, nil)
}

func handleGroupsLeave(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params GroupsLeaveParams
	if !parseParams(req, &params) {
		return
	}
	if err := linkSelfClient.Groups().Leave(ctx, params.GroupID); err != nil {
		sendError(req.ID, -32000, "groups.leave failed", err.Error())
		return
	}
	sendResponse(req.ID, nil)
}

func handleGroupsList(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	groups, err := linkSelfClient.Groups().ListGroups(ctx)
	if err != nil {
		sendError(req.ID, -32000, "groups.list failed", err.Error())
		return
	}
	sendResponse(req.ID, groups)
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
