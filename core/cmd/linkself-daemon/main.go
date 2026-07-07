package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

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
	// CircuitRelays: relay nodes to stay reachable through when this node
	// cannot accept inbound connections (multiaddr with /p2p/...).
	CircuitRelays []string `json:"circuitRelays,omitempty"`
	// EnableRelayService: serve Circuit Relay v2 for unreachable peers
	// (enable on always-on nodes).
	EnableRelayService bool `json:"enableRelayService,omitempty"`
	// ForceReachability: "public", "private" or "" (auto detection).
	ForceReachability string `json:"forceReachability,omitempty"`
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
	Channel   string `json:"channel"`
	GroupID   string `json:"groupID"`
	Retention string `json:"retention,omitempty"` // e.g. "720h" for 30 days; empty = permanent
}

type GroupSharePurgeParams struct {
	Channel string `json:"channel"`
}

type GroupSharePutParams struct {
	Channel  string          `json:"channel"`
	Topic    string          `json:"topic"`
	RecordID string          `json:"recordID"`
	Body     json.RawMessage `json:"body"`
}

type GroupShareGetParams struct {
	Channel  string `json:"channel"`
	RecordID string `json:"recordID"`
}

type GroupShareDeleteParams struct {
	Channel  string `json:"channel"`
	Topic    string `json:"topic"`
	RecordID string `json:"recordID"`
}

type GroupShareSubscribeParams struct {
	Channel string   `json:"channel"`
	Topics  []string `json:"topics"`
}

type GroupShareListParams struct {
	Channel string `json:"channel"`
}

type GroupShareDumpParams struct {
	GroupID string `json:"groupID"`
}

type GroupShareRestoreParams struct {
	Records []*linkself.SharedRecord `json:"records"`
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
	case "mydb.put":
		handleDeviceDBPut(req)
	case "mydb.get":
		handleDeviceDBGet(req)
	case "mydb.delete":
		handleDeviceDBDelete(req)
	case "mydb.list":
		handleDeviceDBList(req)
	case "mydb.dump":
		handleMyDBDump(req)
	case "mydb.restore":
		handleMyDBRestore(req)
	case "network.create":
		handleGroupsCreate(req)
	case "network.addMember":
		handleGroupsAddMember(req)
	case "network.leave":
		handleGroupsLeave(req)
	case "network.list":
		handleGroupsList(req)
	case "network.get":
		handleGroupsGet(req)
	case "generateTestDID":
		handleGenerateTestDID(req)
	case "injectTestMessage":
		handleInjectTestMessage(req)
	case "createPairingToken":
		handleCreatePairingToken(req)
	case "completePairing":
		handleCompletePairing(req)
	case "dangerouslyDeleteAllData":
		handleDangerouslyDeleteAllData(req)
	case "mydb.exec":
		handleDBExec(req)
	case "mydb.query":
		handleDBQuery(req)
	case "mydb.migrate":
		handleDBMigrate(req)
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
		IdentityPath:       params.IdentityPath,
		ListenAddrs:        params.ListenAddrs,
		BootstrapPeers:     params.BootstrapPeers,
		CircuitRelays:      params.CircuitRelays,
		EnableRelayService: params.EnableRelayService,
		ForceReachability:  params.ForceReachability,
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

// --- Test data injection handlers ---

func handleGenerateTestDID(req *JSONRPCRequest) {
	did, err := linkself.GenerateTestDID()
	if err != nil {
		sendError(req.ID, -32000, "generateTestDID failed", err.Error())
		return
	}
	sendResponse(req.ID, map[string]string{"did": did})
}

type InjectTestMessageParams struct {
	FromDID string `json:"fromDID"`
	Payload string `json:"payload"`
}

func handleInjectTestMessage(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params InjectTestMessageParams
	if !parseParams(req, &params) {
		return
	}
	if err := linkSelfClient.InjectTestMessage(ctx, params.FromDID, []byte(params.Payload)); err != nil {
		sendError(req.ID, -32000, "injectTestMessage failed", err.Error())
		return
	}
	sendResponse(req.ID, nil)
}

// --- Pairing handlers ---

type CreatePairingTokenParams struct {
	TTLSeconds int `json:"ttlSeconds"`
}

func handleCreatePairingToken(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params CreatePairingTokenParams
	if !parseParams(req, &params) {
		return
	}
	ttl := time.Duration(params.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	secret, err := linkSelfClient.CreatePairingToken(ctx, ttl)
	if err != nil {
		sendError(req.ID, -32000, "createPairingToken failed", err.Error())
		return
	}
	sendResponse(req.ID, map[string]interface{}{
		"secret":     secret,
		"ttlSeconds": params.TTLSeconds,
	})
}

type CompletePairingParams struct {
	Secret string `json:"secret"`
}

func handleCompletePairing(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params CompletePairingParams
	if !parseParams(req, &params) {
		return
	}
	userPrivKey, err := linkSelfClient.CompletePairing(ctx, params.Secret)
	if err != nil {
		sendError(req.ID, -32000, "completePairing failed", err.Error())
		return
	}
	sendResponse(req.ID, map[string]interface{}{
		"userPrivKey": userPrivKey,
	})
}

// --- DangerouslyDeleteAllData handler ---

func handleDangerouslyDeleteAllData(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	if err := linkSelfClient.DangerouslyDeleteAllData(ctx); err != nil {
		sendError(req.ID, -32000, "dangerouslyDeleteAllData failed", err.Error())
		return
	}
	linkSelfClient = nil
	sendResponse(req.ID, nil)
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
	if err := linkSelfClient.MyDB().Put(ctx, params.Table, params.RecordID, params.Body); err != nil {
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
	rec, err := linkSelfClient.MyDB().Get(ctx, params.Table, params.RecordID)
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
	if err := linkSelfClient.MyDB().Delete(ctx, params.Table, params.RecordID); err != nil {
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
	recs, err := linkSelfClient.MyDB().List(ctx, params.Table)
	if err != nil {
		sendError(req.ID, -32000, "devicedb.list failed", err.Error())
		return
	}
	sendResponse(req.ID, recs)
}

func handleMyDBDump(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	recs, err := linkSelfClient.MyDB().Dump(ctx)
	if err != nil {
		sendError(req.ID, -32000, "mydb.dump failed", err.Error())
		return
	}
	sendResponse(req.ID, recs)
}

type MyDBRestoreParams struct {
	Records []*linkself.Record `json:"records"`
}

func handleMyDBRestore(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params MyDBRestoreParams
	if !parseParams(req, &params) {
		return
	}
	applied, err := linkSelfClient.MyDB().Restore(ctx, params.Records)
	if err != nil {
		sendError(req.ID, -32000, "mydb.restore failed", err.Error())
		return
	}
	sendResponse(req.ID, map[string]int{"applied": applied})
}

// Note: SharedDB RPC handlers removed — SharedDB is no longer a public API.

// --- Groups handlers ---

func handleGroupsCreate(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params GroupsCreateParams
	if !parseParams(req, &params) {
		return
	}
	groupID, err := linkSelfClient.Network().CreateGroup(ctx, params.MemberDIDs)
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
	if err := linkSelfClient.Network().AddMember(ctx, params.GroupID, params.MemberDID); err != nil {
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
	if err := linkSelfClient.Network().Leave(ctx, params.GroupID); err != nil {
		sendError(req.ID, -32000, "groups.leave failed", err.Error())
		return
	}
	sendResponse(req.ID, nil)
}

func handleGroupsList(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	groups, err := linkSelfClient.Network().ListGroups(ctx)
	if err != nil {
		sendError(req.ID, -32000, "groups.list failed", err.Error())
		return
	}
	sendResponse(req.ID, groups)
}

type GroupsGetParams struct {
	GroupID string `json:"groupID"`
}

func handleGroupsGet(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params GroupsGetParams
	if !parseParams(req, &params) {
		return
	}
	members, err := linkSelfClient.Network().GetGroup(ctx, params.GroupID)
	if err != nil {
		sendError(req.ID, -32000, "network.get failed", err.Error())
		return
	}
	sendResponse(req.ID, map[string]interface{}{
		"groupID":    params.GroupID,
		"memberDIDs": members,
	})
}

// --- DB handlers ---

type DBExecParams struct {
	SQL  string        `json:"sql"`
	Args []interface{} `json:"args,omitempty"`
}

func handleDBExec(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params DBExecParams
	if !parseParams(req, &params) {
		return
	}
	result, err := linkSelfClient.MyDB().Exec(ctx, params.SQL, params.Args...)
	if err != nil {
		sendError(req.ID, -32000, "db.exec failed", err.Error())
		return
	}
	rows, _ := result.RowsAffected()
	sendResponse(req.ID, map[string]int64{"rowsAffected": rows})
}

type DBQueryParams struct {
	SQL  string        `json:"sql"`
	Args []interface{} `json:"args,omitempty"`
}

func handleDBQuery(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params DBQueryParams
	if !parseParams(req, &params) {
		return
	}
	rows, err := linkSelfClient.MyDB().Query(ctx, params.SQL, params.Args...)
	if err != nil {
		sendError(req.ID, -32000, "db.query failed", err.Error())
		return
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		sendError(req.ID, -32000, "db.query columns failed", err.Error())
		return
	}

	var results []map[string]interface{}
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			sendError(req.ID, -32000, "db.query scan failed", err.Error())
			return
		}
		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			row[col] = vals[i]
		}
		results = append(results, row)
	}
	sendResponse(req.ID, map[string]interface{}{"columns": cols, "rows": results})
}

type DBMigrateParams struct {
	Migrations []linkself.Migration `json:"migrations"`
}

func handleDBMigrate(req *JSONRPCRequest) {
	if !requireClient(req) {
		return
	}
	var params DBMigrateParams
	if !parseParams(req, &params) {
		return
	}
	if err := linkSelfClient.MyDB().Migrate(ctx, params.Migrations); err != nil {
		sendError(req.ID, -32000, "db.migrate failed", err.Error())
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
