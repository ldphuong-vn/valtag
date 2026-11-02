package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
)

// MockMCPServer provides a mock MCP server for testing
type MockMCPServer struct {
	server          *httptest.Server
	requestLog      []MockRequest
	logMutex        sync.RWMutex
	responseHandler func(req *http.Request) (*http.Response, error)
}

// MockRequest represents a logged MCP server request
type MockRequest struct {
	Method      string
	Path        string
	Body        json.RawMessage
	Headers     http.Header
	RemoteAddr  string
	Timestamp   int64
	QueryParams map[string][]string
}

// MCPRequestPayload represents a standard MCP request
type MCPRequestPayload struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponsePayload represents a standard MCP response
type MCPResponsePayload struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents an MCP error response
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewMockMCPServer creates a new mock MCP server for testing
func NewMockMCPServer() *MockMCPServer {
	m := &MockMCPServer{
		requestLog: make([]MockRequest, 0),
	}

	m.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log request
		bodyBytes := make([]byte, 0)
		if r.Body != nil {
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r.Body)
			bodyBytes = buf.Bytes()
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		m.logMutex.Lock()
		m.requestLog = append(m.requestLog, MockRequest{
			Method:      r.Method,
			Path:        r.URL.Path,
			Body:        bodyBytes,
			Headers:     r.Header.Clone(),
			RemoteAddr:  r.RemoteAddr,
			Timestamp:   int64(len(m.requestLog)),
			QueryParams: r.URL.Query(),
		})
		m.logMutex.Unlock()

		// Use custom handler if provided
		if m.responseHandler != nil {
			resp, err := m.responseHandler(r)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}
			for k, vv := range resp.Header {
				for _, v := range vv {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(resp.StatusCode)
			_, _ = bytes.NewReader(bodyBytes).WriteTo(w)
			return
		}

		// Default handler - parse MCP request
		var mcpReq MCPRequestPayload
		if err := json.Unmarshal(bodyBytes, &mcpReq); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(MCPResponsePayload{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &MCPError{
					Code:    -32700,
					Message: "Parse error",
				},
			})
			return
		}

		// Handle different MCP methods
		response := m.handleMCPMethod(r.Context(), mcpReq)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))

	return m
}

// handleMCPMethod handles MCP method calls
func (m *MockMCPServer) handleMCPMethod(ctx context.Context, req MCPRequestPayload) MCPResponsePayload {
	switch req.Method {
	case "tools/list":
		return m.handleToolsList(req)
	case "resources/list":
		return m.handleResourcesList(req)
	case "tools/call":
		return m.handleToolCall(req)
	case "initialize":
		return m.handleInitialize(req)
	default:
		return MCPResponsePayload{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

// handleInitialize handles MCP initialize request
func (m *MockMCPServer) handleInitialize(req MCPRequestPayload) MCPResponsePayload {
	result := map[string]interface{}{
		"serverInfo": map[string]string{
			"name":    "mock-mcp-server",
			"version": "1.0.0",
		},
		"capabilities": map[string]interface{}{
			"tools":     map[string]interface{}{},
			"resources": map[string]interface{}{},
		},
	}

	resultJSON, _ := json.Marshal(result)
	return MCPResponsePayload{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}
}

// handleToolsList handles tools/list MCP request
func (m *MockMCPServer) handleToolsList(req MCPRequestPayload) MCPResponsePayload {
	tools := []map[string]interface{}{
		{
			"name":        "read_secret",
			"description": "Reads a secret value from valtag",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"secret_id": map[string]string{
						"type":        "string",
						"description": "The ID of the secret to read",
					},
				},
				"required": []string{"secret_id"},
			},
		},
		{
			"name":        "request_access",
			"description": "Requests access to a secret with optional parameters",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"secret_id": map[string]string{
						"type":        "string",
						"description": "The ID of the secret",
					},
					"duration_minutes": map[string]string{
						"type":        "string",
						"description": "Duration in minutes",
					},
					"reason": map[string]string{
						"type":        "string",
						"description": "Reason for access request",
					},
				},
				"required": []string{"secret_id"},
			},
		},
	}

	result := map[string]interface{}{
		"tools": tools,
	}

	resultJSON, _ := json.Marshal(result)
	return MCPResponsePayload{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}
}

// handleResourcesList handles resources/list MCP request
func (m *MockMCPServer) handleResourcesList(req MCPRequestPayload) MCPResponsePayload {
	resources := []map[string]interface{}{
		{
			"uri":      "valtag://secrets/list",
			"name":     "List Secrets",
			"mimeType": "application/json",
		},
	}

	result := map[string]interface{}{
		"resources": resources,
	}

	resultJSON, _ := json.Marshal(result)
	return MCPResponsePayload{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  resultJSON,
	}
}

// handleToolCall handles tools/call MCP request
func (m *MockMCPServer) handleToolCall(req MCPRequestPayload) MCPResponsePayload {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(req.Params, &params); err != nil {
		return MCPResponsePayload{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	var result json.RawMessage
	var errResp *MCPError

	switch params.Name {
	case "read_secret":
		result, errResp = m.handleReadSecret(params.Arguments)
	case "request_access":
		result, errResp = m.handleRequestAccess(params.Arguments)
	default:
		errResp = &MCPError{
			Code:    -32601,
			Message: fmt.Sprintf("Tool not found: %s", params.Name),
		}
	}

	return MCPResponsePayload{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
		Error:   errResp,
	}
}

// handleReadSecret mocks reading a secret
func (m *MockMCPServer) handleReadSecret(args json.RawMessage) (json.RawMessage, *MCPError) {
	var params struct {
		SecretID string `json:"secret_id"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, &MCPError{
			Code:    -32602,
			Message: "Invalid arguments",
		}
	}

	if params.SecretID == "" {
		return nil, &MCPError{
			Code:    -32602,
			Message: "secret_id is required",
		}
	}

	result := map[string]interface{}{
		"secret_id": params.SecretID,
		"value":     "mock-secret-value-" + params.SecretID,
		"metadata": map[string]interface{}{
			"created_at": "2024-01-01T00:00:00Z",
			"version":    1,
		},
	}

	resultJSON, _ := json.Marshal(result)
	return resultJSON, nil
}

// handleRequestAccess mocks requesting access to a secret
func (m *MockMCPServer) handleRequestAccess(args json.RawMessage) (json.RawMessage, *MCPError) {
	var params struct {
		SecretID        string `json:"secret_id"`
		DurationMinutes int    `json:"duration_minutes,omitempty"`
		Reason          string `json:"reason,omitempty"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, &MCPError{
			Code:    -32602,
			Message: "Invalid arguments",
		}
	}

	if params.SecretID == "" {
		return nil, &MCPError{
			Code:    -32602,
			Message: "secret_id is required",
		}
	}

	result := map[string]interface{}{
		"request_id":       "req-" + params.SecretID,
		"status":           "pending",
		"secret_id":        params.SecretID,
		"duration_minutes": params.DurationMinutes,
		"reason":           params.Reason,
		"created_at":       "2024-01-01T00:00:00Z",
		"expires_at":       "2024-01-01T01:00:00Z",
	}

	resultJSON, _ := json.Marshal(result)
	return resultJSON, nil
}

// URL returns the mock server's base URL
func (m *MockMCPServer) URL() string {
	return m.server.URL
}

// GetRequestLog returns all logged requests
func (m *MockMCPServer) GetRequestLog() []MockRequest {
	m.logMutex.RLock()
	defer m.logMutex.RUnlock()

	// Create a copy to avoid race conditions
	log := make([]MockRequest, len(m.requestLog))
	copy(log, m.requestLog)
	return log
}

// ClearRequestLog clears the request log
func (m *MockMCPServer) ClearRequestLog() {
	m.logMutex.Lock()
	defer m.logMutex.Unlock()
	m.requestLog = make([]MockRequest, 0)
}

// RequestCount returns the number of requests logged
func (m *MockMCPServer) RequestCount() int {
	m.logMutex.RLock()
	defer m.logMutex.RUnlock()
	return len(m.requestLog)
}

// SetResponseHandler sets a custom response handler
func (m *MockMCPServer) SetResponseHandler(handler func(req *http.Request) (*http.Response, error)) {
	m.responseHandler = handler
}

// Close shuts down the mock server
func (m *MockMCPServer) Close() {
	if m.server != nil {
		m.server.Close()
	}
}

// CallTool makes a tool call to the mock MCP server
func (m *MockMCPServer) CallTool(ctx context.Context, toolName string, args interface{}) (json.RawMessage, error) {
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal arguments: %w", err)
	}

	req := MCPRequestPayload{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: json.RawMessage(fmt.Sprintf(`{
			"name": %q,
			"arguments": %s
		}`, toolName, string(argsJSON))),
	}

	reqJSON, _ := json.Marshal(req)
	resp, err := http.Post(m.URL()+"/", "application/json", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to call tool: %w", err)
	}
	defer resp.Body.Close()

	var mcpResp MCPResponsePayload
	if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if mcpResp.Error != nil {
		return nil, fmt.Errorf("MCP error: %s (code: %d)", mcpResp.Error.Message, mcpResp.Error.Code)
	}

	return mcpResp.Result, nil
}

// ListTools gets the list of available tools from the mock server
func (m *MockMCPServer) ListTools(ctx context.Context) ([]map[string]interface{}, error) {
	req := MCPRequestPayload{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  json.RawMessage("{}"),
	}

	reqJSON, _ := json.Marshal(req)
	resp, err := http.Post(m.URL()+"/", "application/json", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}
	defer resp.Body.Close()

	var mcpResp MCPResponsePayload
	if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if mcpResp.Error != nil {
		return nil, fmt.Errorf("MCP error: %s", mcpResp.Error.Message)
	}

	var result struct {
		Tools []map[string]interface{} `json:"tools"`
	}
	if err := json.Unmarshal(mcpResp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tools: %w", err)
	}

	return result.Tools, nil
}
