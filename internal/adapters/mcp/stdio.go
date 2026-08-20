package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// dispatcher answers one JSON-RPC message. The second return value is false
// for messages that must not be answered at all (notifications).
//
// Two implement it: Server, which runs the tools in this process, and Proxy,
// which forwards them to a self-hosted server. The stdio loop below is shared
// between them.
type dispatcher func(ctx context.Context, req rpcRequest) (rpcResponse, bool)

// ServeStdio runs the MCP server over a newline-delimited JSON-RPC stream.
//
// This is the transport an installed MCP bundle uses: the chat client spawns
// the binary as a child process and talks to it over the pipe, so there is no
// network, no port and no bearer token in play — the OS process boundary is
// the trust boundary. The Streamable HTTP transport (Register) stays the
// self-hosted path; both share dispatch, so the tool surface is identical.
//
// Nothing but protocol messages may ever reach out: the caller is responsible
// for pointing its logger at stderr before calling this.
//
// It returns when in reaches EOF, when ctx is canceled, or on a stream error.
func (s *Server) ServeStdio(ctx context.Context, in io.Reader, out io.Writer) error {
	return serveStdio(ctx, in, out, s.logger, s.dispatch)
}

// serveStdio is the transport loop both ServeStdio implementations share.
func serveStdio(ctx context.Context, in io.Reader, out io.Writer, logger *slog.Logger, dispatch dispatcher) error {
	requests, readErr := readStdio(ctx, in)

	enc := json.NewEncoder(out)
	var writeMu sync.Mutex

	// Requests are handled concurrently: a fast tool call blocks on a provider
	// round-trip, and a client that pipelines a ping behind it should not have
	// to wait for that. Writes are serialized so messages never interleave.
	var wg sync.WaitGroup
	defer wg.Wait()

	for {
		select {
		case <-ctx.Done():
			return nil

		case req, ok := <-requests:
			if !ok {
				return <-readErr
			}

			wg.Add(1)
			go func() {
				defer wg.Done()

				resp, wantsReply := dispatch(ctx, req)
				if !wantsReply {
					return
				}

				writeMu.Lock()
				defer writeMu.Unlock()
				if err := enc.Encode(resp); err != nil {
					logger.Error("writing stdio response", "method", req.Method, "error", err)
				}
			}()
		}
	}
}

// readStdio decodes messages off the input pipe on its own goroutine, so the
// serve loop can still honor ctx while a read is parked on an idle pipe.
//
// The error channel is buffered and closed-on-send: a nil value means the
// client closed the pipe, which is an ordinary shutdown, not a failure.
func readStdio(ctx context.Context, in io.Reader) (<-chan rpcRequest, <-chan error) {
	requests := make(chan rpcRequest)
	readErr := make(chan error, 1)

	go func() {
		defer close(requests)

		dec := json.NewDecoder(in)
		for {
			var req rpcRequest
			if err := dec.Decode(&req); err != nil {
				if !errors.Is(err, io.EOF) {
					// A JSON syntax error leaves the decoder mid-value with no
					// way to resynchronize, so the stream ends here.
					readErr <- fmt.Errorf("reading stdio request: %w", err)
					return
				}
				readErr <- nil
				return
			}

			select {
			case requests <- req:
			case <-ctx.Done():
				readErr <- nil
				return
			}
		}
	}()

	return requests, readErr
}
