package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDaemonJSONRPC tests the daemon's JSON-RPC communication
func TestDaemonJSONRPC(t *testing.T) {
	// Build the daemon binary
	daemonPath := filepath.Join(t.TempDir(), "linkself-daemon-test")
	buildCmd := exec.Command("go", "build", "-o", daemonPath, "main.go")
	buildCmd.Dir = filepath.Dir(".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build daemon: %v", err)
	}

	// Start the daemon process
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, daemonPath)
	cmd.Dir = t.TempDir() // Use temp dir for identity file

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Create a scanner for stdout
	scanner := bufio.NewScanner(stdout)
	responseChan := make(chan JSONRPCResponse, 1)

	// Start reading responses in a goroutine
	go func() {
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			var resp JSONRPCResponse
			if err := json.Unmarshal([]byte(line), &resp); err == nil {
				responseChan <- resp
			}
		}
	}()

	// Test 1: Start the daemon
	t.Run("Start", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "start",
			Params:  StartParams{},
			ID:      1,
		}

		reqJSON, _ := json.Marshal(req)
		if _, err := stdin.Write([]byte(string(reqJSON) + "\n")); err != nil {
			t.Fatalf("Failed to write request: %v", err)
		}

		select {
		case resp := <-responseChan:
			if resp.Error != nil {
				t.Fatalf("Start failed: %v", resp.Error)
			}
			// Check response ID (may be float64 from JSON unmarshaling)
			respID, ok := resp.ID.(float64)
			if !ok {
				respIDInt, ok := resp.ID.(int)
				if !ok || respIDInt != 1 {
					t.Fatalf("Unexpected response ID: %v (type: %T)", resp.ID, resp.ID)
				}
			} else if int(respID) != 1 {
				t.Fatalf("Unexpected response ID: %v", resp.ID)
			}

			// Parse result
			resultBytes, _ := json.Marshal(resp.Result)
			var result StartResult
			if err := json.Unmarshal(resultBytes, &result); err != nil {
				t.Fatalf("Failed to parse result: %v", err)
			}

			if result.DID == "" {
				t.Error("DID is empty")
			}
			t.Logf("Started daemon with DID: %s", result.DID)

		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for start response")
		}
	})

	// Test 2: Get My DID
	t.Run("GetMyDID", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "getMyDID",
			ID:      2,
		}

		reqJSON, _ := json.Marshal(req)
		if _, err := stdin.Write([]byte(string(reqJSON) + "\n")); err != nil {
			t.Fatalf("Failed to write request: %v", err)
		}

		select {
		case resp := <-responseChan:
			if resp.Error != nil {
				t.Fatalf("GetMyDID failed: %v", resp.Error)
			}
			// Check response ID
			respID, ok := resp.ID.(float64)
			if !ok {
				respIDInt, ok := resp.ID.(int)
				if !ok || respIDInt != 2 {
					t.Fatalf("Unexpected response ID: %v (type: %T)", resp.ID, resp.ID)
				}
			} else if int(respID) != 2 {
				t.Fatalf("Unexpected response ID: %v", resp.ID)
			}

			did, ok := resp.Result.(string)
			if !ok {
				t.Fatalf("Unexpected result type: %T", resp.Result)
			}
			if did == "" {
				t.Error("DID is empty")
			}
			t.Logf("My DID: %s", did)

		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for getMyDID response")
		}
	})

	// Test 3: GenerateTestDID
	t.Run("GenerateTestDID", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "generateTestDID",
			ID:      10,
		}

		reqJSON, _ := json.Marshal(req)
		if _, err := stdin.Write([]byte(string(reqJSON) + "\n")); err != nil {
			t.Fatalf("Failed to write request: %v", err)
		}

		select {
		case resp := <-responseChan:
			if resp.Error != nil {
				t.Fatalf("generateTestDID failed: %v", resp.Error)
			}
			resultBytes, _ := json.Marshal(resp.Result)
			var result map[string]string
			if err := json.Unmarshal(resultBytes, &result); err != nil {
				t.Fatalf("Failed to parse result: %v", err)
			}
			did := result["did"]
			if !strings.HasPrefix(did, "did:test:") {
				t.Errorf("expected did:test: prefix, got %q", did)
			}
			t.Logf("Generated test DID: %s", did)

		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for generateTestDID response")
		}
	})

	// Test 4: InjectTestMessage
	t.Run("InjectTestMessage", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "injectTestMessage",
			Params: map[string]string{
				"fromDID": "did:test:0000000000000001",
				"payload": "hello from test",
			},
			ID: 11,
		}

		reqJSON, _ := json.Marshal(req)
		if _, err := stdin.Write([]byte(string(reqJSON) + "\n")); err != nil {
			t.Fatalf("Failed to write request: %v", err)
		}

		select {
		case resp := <-responseChan:
			if resp.Error != nil {
				t.Fatalf("injectTestMessage failed: %v", resp.Error)
			}
			t.Log("injectTestMessage succeeded")

		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for injectTestMessage response")
		}
	})

	// Test 5: InjectTestMessage rejects real DID
	t.Run("InjectTestMessage_rejects_real_DID", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "injectTestMessage",
			Params: map[string]string{
				"fromDID": "did:key:z6MkpTHR8VNs5fA2",
				"payload": "should fail",
			},
			ID: 12,
		}

		reqJSON, _ := json.Marshal(req)
		if _, err := stdin.Write([]byte(string(reqJSON) + "\n")); err != nil {
			t.Fatalf("Failed to write request: %v", err)
		}

		// Drain responses until we get one with matching ID (skip notifications).
		deadline := time.After(5 * time.Second)
		for {
			select {
			case resp := <-responseChan:
				// Skip notifications (ID == nil or non-matching).
				respID, _ := resp.ID.(float64)
				if int(respID) != 12 {
					continue
				}
				if resp.Error == nil {
					t.Fatal("Expected error when injecting with real DID")
				}
				t.Logf("Correctly rejected real DID: %s", resp.Error.Message)
				return
			case <-deadline:
				t.Fatal("Timeout waiting for error response")
			}
		}
	})

	// Test 6: Stop the daemon
	t.Run("Stop", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "stop",
			ID:      3,
		}

		reqJSON, _ := json.Marshal(req)
		if _, err := stdin.Write([]byte(string(reqJSON) + "\n")); err != nil {
			t.Fatalf("Failed to write request: %v", err)
		}

		select {
		case resp := <-responseChan:
			if resp.Error != nil {
				t.Fatalf("Stop failed: %v", resp.Error)
			}
			// Check response ID
			respID, ok := resp.ID.(float64)
			if !ok {
				respIDInt, ok := resp.ID.(int)
				if !ok || respIDInt != 3 {
					t.Fatalf("Unexpected response ID: %v (type: %T)", resp.ID, resp.ID)
				}
			} else if int(respID) != 3 {
				t.Fatalf("Unexpected response ID: %v", resp.ID)
			}
			t.Log("Daemon stopped successfully")

		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for stop response")
		}
	})
}

// TestDaemonErrorHandling tests error handling
func TestDaemonErrorHandling(t *testing.T) {
	daemonPath := filepath.Join(t.TempDir(), "linkself-daemon-test")
	buildCmd := exec.Command("go", "build", "-o", daemonPath, "main.go")
	buildCmd.Dir = filepath.Dir(".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build daemon: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, daemonPath)
	cmd.Dir = t.TempDir()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon: %v", err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	scanner := bufio.NewScanner(stdout)
	responseChan := make(chan JSONRPCResponse, 1)

	go func() {
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var resp JSONRPCResponse
			if err := json.Unmarshal([]byte(line), &resp); err == nil {
				responseChan <- resp
			}
		}
	}()

	// Test invalid method
	t.Run("InvalidMethod", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "invalidMethod",
			ID:      1,
		}

		reqJSON, _ := json.Marshal(req)
		if _, err := stdin.Write([]byte(string(reqJSON) + "\n")); err != nil {
			t.Fatalf("Failed to write request: %v", err)
		}

		select {
		case resp := <-responseChan:
			if resp.Error == nil {
				t.Error("Expected error for invalid method")
			}
			if resp.Error.Code != -32601 {
				t.Errorf("Unexpected error code: %d", resp.Error.Code)
			}

		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for error response")
		}
	})

	// Test getMyDID without starting
	t.Run("GetMyDIDWithoutStart", func(t *testing.T) {
		req := JSONRPCRequest{
			JSONRPC: "2.0",
			Method:  "getMyDID",
			ID:      2,
		}

		reqJSON, _ := json.Marshal(req)
		if _, err := stdin.Write([]byte(string(reqJSON) + "\n")); err != nil {
			t.Fatalf("Failed to write request: %v", err)
		}

		select {
		case resp := <-responseChan:
			if resp.Error == nil {
				t.Error("Expected error when calling getMyDID without start")
			}
			if resp.Error.Code != -32000 {
				t.Errorf("Unexpected error code: %d", resp.Error.Code)
			}

		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for error response")
		}
	})
}
