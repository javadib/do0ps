// Package mcp is the primary (driving) adapter: it turns MCP tool calls into
// calls on the application use cases and translates results and errors back.
//
// Everything transport-shaped lives here — JSON Schema, JSON-RPC framing,
// Fiber routing. Use cases in internal/core/app know none of it.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/sse"

	"github.com/javadib/do0ps/internal/core/domain"
)

// Tool is one MCP tool exposed to the calling model.
//
// InputSchema quality is what lets a chatbot turn "a 2GB server in Tehran"
// into structured parameters, so every property carries a description with
// units and an example.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`

	Handler func(ctx context.Context, args json.RawMessage) (any, error) `json:"-"`
}

// Server holds the tool registry and dispatches calls to it.
type Server struct {
	logger *slog.Logger
	tools  []Tool
	index  map[string]int

	// heartbeatInterval overrides the SSE heartbeat used to detect a
	// disconnected client. Zero means "let the sse middleware use its
	// default" (see WithHeartbeatInterval).
	heartbeatInterval time.Duration
}

// Option configures a Server.
type Option func(*Server) error

// WithLogger sets the logger used for tool failures.
func WithLogger(l *slog.Logger) Option {
	return func(s *Server) error {
		if l == nil {
			return errors.New("logger must not be nil")
		}
		s.logger = l
		return nil
	}
}

// WithHeartbeatInterval overrides the SSE stream's heartbeat interval, used
// to bound how quickly a disconnected client is noticed. Production traffic
// is fine with the middleware default (15s); tests set this shorter so a
// closed connection is detected without a slow test run.
func WithHeartbeatInterval(d time.Duration) Option {
	return func(s *Server) error {
		if d <= 0 {
			return errors.New("heartbeat interval must be positive")
		}
		s.heartbeatInterval = d
		return nil
	}
}

// NewServer builds the adapter around a set of tools.
func NewServer(tools []Tool, opts ...Option) (*Server, error) {
	s := &Server{
		logger: slog.Default(),
		index:  make(map[string]int, len(tools)),
	}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil, fmt.Errorf("configuring mcp server: %w", err)
		}
	}

	for _, tool := range tools {
		if tool.Name == "" || tool.Handler == nil {
			return nil, fmt.Errorf("tool %q must have a name and a handler", tool.Name)
		}
		if _, exists := s.index[tool.Name]; exists {
			return nil, fmt.Errorf("duplicate tool %q", tool.Name)
		}
		s.index[tool.Name] = len(s.tools)
		s.tools = append(s.tools, tool)
	}
	return s, nil
}

// Tools returns the registered tools in registration order.
func (s *Server) Tools() []Tool { return s.tools }

// Call runs one tool by name.
func (s *Server) Call(ctx context.Context, name string, args json.RawMessage) (any, error) {
	i, ok := s.index[name]
	if !ok {
		return nil, fmt.Errorf("unknown tool %q: %w", name, domain.ErrInvalidInput)
	}
	return s.tools[i].Handler(ctx, args)
}

// Register mounts the MCP endpoint on a Fiber router.
//
// Streamable HTTP has two halves on the same path: POST answers tools/list
// and tools/call directly with a JSON-RPC response, and GET opens an SSE
// stream (via github.com/gofiber/fiber/v3/middleware/sse, per AGENTS.md 5)
// for server-initiated messages. No use case pushes anything down the stream
// yet, so it currently just proves the transport round-trip.
func (s *Server) Register(router fiber.Router) {
	router.Post("/mcp", s.handleRPC)
	router.Get("/mcp", sse.New(sse.Config{
		Handler:           s.handleStream,
		HeartbeatInterval: s.heartbeatInterval,
		OnClose: func(_ fiber.Ctx, err error) {
			if err != nil && !errors.Is(err, context.Canceled) {
				s.logger.Error("mcp stream closed", "error", err)
			}
		},
	}))
}

// handleStream serves the SSE half of Streamable HTTP: a long-lived
// connection a client can open to receive server-initiated messages. It
// sends a "ready" event so a client (or test) can confirm the stream is live,
// then holds the connection open until the client disconnects — surfaced
// through stream.Context() being canceled, per AGENTS.md 5.
func (s *Server) handleStream(_ fiber.Ctx, stream *sse.Stream) error {
	if err := stream.Event(sse.Event{
		Name: "ready",
		Data: fiber.Map{"protocol": "mcp-streamable-http"},
	}); err != nil {
		return err
	}
	<-stream.Context().Done()
	return nil
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC 2.0 error codes used here.
const (
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInvalidParams  = -32602
	codeInternalError  = -32603
)

type callParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s *Server) handleRPC(c fiber.Ctx) error {
	var req rpcRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(errorResponse(nil, codeInvalidRequest, "malformed JSON-RPC request"))
	}

	switch req.Method {
	case "tools/list":
		return c.JSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: fiber.Map{"tools": s.tools}})

	case "tools/call":
		var params callParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return c.JSON(errorResponse(req.ID, codeInvalidParams, "malformed tool call parameters"))
		}

		result, err := s.Call(c.Context(), params.Name, params.Arguments)
		if err != nil {
			return c.JSON(s.toolFailure(req.ID, params.Name, err))
		}
		return c.JSON(rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: toolResult(result)})

	default:
		return c.JSON(errorResponse(req.ID, codeMethodNotFound, "unsupported method "+req.Method))
	}
}

// toolFailure reports a failed tool call. Input problems come back as protocol
// errors the caller can act on; anything else is logged in full and reported
// without internal detail.
func (s *Server) toolFailure(id json.RawMessage, name string, err error) rpcResponse {
	switch {
	case errors.Is(err, domain.ErrInvalidInput),
		errors.Is(err, domain.ErrInvalidCredentials),
		errors.Is(err, domain.ErrNotFound):
		return errorResponse(id, codeInvalidParams, err.Error())
	default:
		s.logger.Error("tool call failed", "tool", name, "error", err)
		return errorResponse(id, codeInternalError, "tool "+name+" failed")
	}
}

func errorResponse(id json.RawMessage, code int, msg string) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

// toolResult wraps a use case result in MCP tool content.
func toolResult(v any) fiber.Map {
	encoded, err := json.Marshal(v)
	if err != nil {
		return fiber.Map{
			"content": []fiber.Map{{"type": "text", "text": fmt.Sprintf("encoding result: %v", err)}},
			"isError": true,
		}
	}
	return fiber.Map{
		"content": []fiber.Map{{"type": "text", "text": string(encoded)}},
	}
}
