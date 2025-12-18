package metrics

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// Endpoint wraps a TCP listener that serves a simple JSON payload.
type Endpoint struct {
	addr     string
	payload  func() ([]byte, error)
	listener net.Listener
	running  bool
	mu       sync.Mutex
}

// NewEndpoint creates a new metrics endpoint at addr.
func NewEndpoint(addr string, payload func() ([]byte, error)) *Endpoint {
	return &Endpoint{
		addr:    addr,
		payload: payload,
	}
}

// Start begins listening and serving payloads.
func (e *Endpoint) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	ln, err := net.Listen("tcp", e.addr)
	if err != nil {
		return fmt.Errorf("metrics endpoint failed on %s: %w", e.addr, err)
	}

	e.listener = ln
	e.running = true

	go e.serve()
	return nil
}

func (e *Endpoint) serve() {
	for {
		conn, err := e.listener.Accept()
		if err != nil {
			e.mu.Lock()
			if !e.running {
				e.mu.Unlock()
				return
			}
			e.mu.Unlock()
			time.Sleep(500 * time.Millisecond)
			continue
		}
		go e.handleConn(conn)
	}
}

func (e *Endpoint) handleConn(conn net.Conn) {
	defer conn.Close()
	if e.payload == nil {
		return
	}
	data, err := e.payload()
	if err != nil {
		conn.Write([]byte(fmt.Sprintf("{\"error\":\"%v\"}", err)))
		return
	}
	conn.Write(data)
}

// Stop closes the listener.
func (e *Endpoint) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}
	e.running = false
	return e.listener.Close()
}
