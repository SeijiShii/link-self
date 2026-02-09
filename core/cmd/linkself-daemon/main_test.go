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

	// Test 3: Stop the daemon
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
