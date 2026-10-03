package server

import (
	"context"
	"fmt"
	"net"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// ownTransportTask seals handler/reader admission with the same drain boundary.
// A parent must register children before launching them; shutdown joins the
// sealed set before closing storage, including work still returning from world.
func (s *Server) ownTransportTask() (func(), bool) {
	s.transportMu.Lock()
	defer s.transportMu.Unlock()
	if s.draining.Load() {
		return nil, false
	}
	s.transportTasks.Add(1)
	return s.transportTasks.Done, true
}

func (s *Server) startTransportTask(task func()) bool {
	done, admitted := s.ownTransportTask()
	if !admitted {
		return false
	}
	go func() { defer done(); task() }()
	return true
}

// Called during startup under lifecycleMu, after the listener is bound.
func (s *Server) serveWebTransport(conn net.PacketConn) {
	s.wtDone = make(chan struct{})
	s.wtServer.H3.ConnContext = s.trackQUICConnection
	go func() {
		defer close(s.wtDone)
		if err := s.wtServer.Serve(conn); err != nil {
			s.reportServeFailure(fmt.Errorf("WebTransport serve: %w", err))
		}
	}()
}

// The pinned HTTP/3 Close only stops listeners. Own accepted connections too,
// so local session cancellation cannot leave a peer waiting for its idle timeout.
func (s *Server) trackQUICConnection(ctx context.Context, conn quic.Connection) context.Context {
	s.transportMu.Lock()
	if s.draining.Load() {
		s.transportMu.Unlock()
		_ = conn.CloseWithError(quic.ApplicationErrorCode(http3.ErrCodeNoError), "server shutting down")
		return ctx
	}
	if s.quicConnections == nil {
		s.quicConnections = make(map[quic.Connection]struct{})
	}
	s.quicConnections[conn] = struct{}{}
	s.transportMu.Unlock()
	context.AfterFunc(conn.Context(), func() {
		s.transportMu.Lock()
		delete(s.quicConnections, conn)
		s.transportMu.Unlock()
	})
	return ctx
}

func (s *Server) closeQUICConnections() {
	s.transportMu.Lock()
	connections := make([]quic.Connection, 0, len(s.quicConnections))
	for conn := range s.quicConnections {
		connections = append(connections, conn)
	}
	s.transportMu.Unlock()
	for _, conn := range connections {
		_ = conn.CloseWithError(quic.ApplicationErrorCode(http3.ErrCodeNoError), "server shutting down")
	}
}
