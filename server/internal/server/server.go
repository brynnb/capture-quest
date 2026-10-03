package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"capturequest/internal/api"
	"capturequest/internal/cache"
	"capturequest/internal/cert"
	"capturequest/internal/config"
	"capturequest/internal/db"
	"capturequest/internal/discordchat"
	"capturequest/internal/logutil"
	"capturequest/internal/overworldoverview"
	"capturequest/internal/session"
	"capturequest/internal/world"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

// worldRuntime is the transport-facing boundary of the gameplay runtime.
type worldRuntime interface {
	HandlePacket(*session.Session, []byte)
	RemoveSession(int)
	ShutdownContext(context.Context) error
}

// Server hosts HTTP, WebSocket and WebTransport connections.
type Server struct {
	ready             atomic.Bool
	draining          atomic.Bool
	failed            atomic.Bool
	failureOnce       sync.Once
	failures          chan error
	lifecycleMu       sync.Mutex
	stopOnce          sync.Once
	stopDone          chan struct{}
	stopErr           error
	started, stopping bool
	httpServer        *http.Server
	httpDone          chan struct{}
	httpCancel        context.CancelFunc
	httpHandlerMu     sync.Mutex
	httpHandlers      sync.WaitGroup
	database          *sql.DB
	wtServer          *webtransport.Server
	wtDone            chan struct{}
	transportMu       sync.Mutex
	transportTasks    sync.WaitGroup
	quicConnections   map[quic.Connection]struct{}
	worldHandler      worldRuntime
	sessionManager    *session.SessionManager
	sessions          map[int]*webtransport.Session
	sessionsMu        sync.Mutex // Protects sessions map
	udpConn           *net.UDPConn
	gracePeriod       time.Duration
	debugMode         bool
	discordChat       *discordchat.Bridge
}

// NewServer constructs a new Server.
func NewServer(ctx context.Context, dsn string, gracePeriod time.Duration, debugMode bool) (*Server, error) {
	sessionManager := session.NewSessionManager()
	session.InitSessionManager(sessionManager)
	worldHandler, err := world.NewWorldHandler(ctx, sessionManager)
	if err != nil {
		return nil, fmt.Errorf("initialize world: %w", err)
	}
	constructed := false
	defer func() {
		if !constructed {
			worldHandler.Shutdown()
		}
	}()

	if err := cache.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	srv := &Server{
		database:       db.GlobalWorldDB.DB,
		worldHandler:   worldHandler,
		sessionManager: sessionManager,
		sessions:       make(map[int]*webtransport.Session),
		gracePeriod:    gracePeriod,
		debugMode:      debugMode,
	}
	discordBridge, err := discordchat.NewFromEnvironment(worldHandler.BroadcastExternalChat)
	if err != nil {
		return nil, fmt.Errorf("failed to configure Discord chat bridge: %w", err)
	}
	if discordBridge != nil {
		srv.discordChat = discordBridge
		worldHandler.SetPublicChatSink(func(message world.ChatMessageBroadcast) {
			discordBridge.Enqueue(discordchat.Message{SenderName: message.SenderName, Text: message.Text})
		})
	}
	constructed = true
	return srv, nil
}

// StartServer configures TLS, QUIC, HTTP, and begins serving WebTransport.
func (s *Server) StartServer() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.stopping || s.started {
		return fmt.Errorf("server cannot start twice or after shutdown")
	}
	s.started = true
	InitLogging()
	if s.discordChat != nil {
		s.discordChat.Start()
		log.Printf("[DiscordChat] Game-chat bridge enabled")
	}
	// TLS
	tlsConf, certManager, err := cert.LoadTLSConfig()
	if err != nil {
		return fmt.Errorf("load TLS config: %w", err)
	}

	// Bind UDP for WebTransport. Local dev can override WT_PORT when the
	// default port is already occupied by another running server.
	wtPort := envPort("WT_PORT", 4433)
	udpConn, port, err := listenUDP(wtPort)
	if err != nil {
		return fmt.Errorf("listen UDP port %d: %w", wtPort, err)
	}
	s.udpConn = udpConn
	log.Printf("WebTransport bound to UDP port: %d", port)

	// QUIC - increased idle timeout for idle game (not real-time 3D MMO)
	quicConf := &quic.Config{
		MaxStreamReceiveWindow:     4 * 1024 * 1024,
		MaxConnectionReceiveWindow: 16 * 1024 * 1024,
		MaxIncomingStreams:         1000,
		MaxIdleTimeout:             5 * time.Minute, // Longer timeout for idle game
	}

	// Create separate mux for WebTransport
	wtMux := http.NewServeMux()
	wtMux.HandleFunc("/cq", s.makeCaptureQuestHandler())

	// Configure TLS for WebTransport
	wtTLSConfig := tlsConf.Clone()
	wtTLSConfig.NextProtos = []string{"h3"}

	// Log the SHA-256 (base64) of the leaf certificate so devs can pin it via VITE_WT_CERT_HASH
	if certManager != nil {
		fmt.Printf("WT certificate SHA-256 (base64): %s\n", certManager.GetHash())
		fmt.Println("Set VITE_WT_CERT_HASH to the value above for local dev pinning.")
	} else if len(wtTLSConfig.Certificates) > 0 && len(wtTLSConfig.Certificates[0].Certificate) > 0 {
		leafDER := wtTLSConfig.Certificates[0].Certificate[0]
		sum := sha256.Sum256(leafDER)
		fmt.Printf("WT certificate SHA-256 (base64): %s\n", b64.StdEncoding.EncodeToString(sum[:]))
		fmt.Println("Set VITE_WT_CERT_HASH to the value above for local dev pinning.")
	} else {
		log.Printf("Warning: no certificate loaded in TLS config; WebTransport will fail.")
	}

	// WebTransport server - no Addr needed since we use Serve() with pre-bound UDP socket
	s.wtServer = &webtransport.Server{
		H3: http3.Server{
			TLSConfig:       wtTLSConfig,
			EnableDatagrams: true,
			QUICConfig:      quicConf,
			Handler:         wtMux,
		},
		CheckOrigin: func(r *http.Request) bool {
			logutil.Debugf("CheckOrigin called for: %s", r.Host)
			return true
		},
	}

	// HTTP handler for OAuth, etc.
	cfg, err := config.Get()
	if err != nil {
		return err
	}
	if err := s.startHTTPServer(tlsConf, certManager, cfg.HTTPPort, port); err != nil {
		return err
	}

	s.serveWebTransport(udpConn)
	s.ready.Store(true)
	return nil
}

func envPort(name string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 {
		return fallback
	}
	return port
}

// makeCaptureQuestHandler upgrades HTTP to WebTransport and manages session lifecycles.
func (s *Server) makeCaptureQuestHandler() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		done, admitted := s.ownTransportTask()
		if !admitted {
			http.Error(rw, "Server shutting down", http.StatusServiceUnavailable)
			return
		}
		defer done()
		// The shipped browser reconnects by authenticating a fresh connection.
		// Never attach an existing authenticated session using its ID or IP.
		if r.URL.Query().Get("sid") != "" && r.URL.Query().Get("sid") != "0" {
			http.Error(rw, "Reconnect requires authentication on a new connection", http.StatusBadRequest)
			return
		}
		sess, err := s.wtServer.Upgrade(rw, r)
		if err != nil {
			log.Printf("WebTransport upgrade error: %v", err)
			return
		}
		clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		sessObj := s.sessionManager.CreateNextSession(s, clientIP, nil)
		if sessObj == nil {
			_ = sess.CloseWithError(0, "server shutting down")
			return
		}
		s.sessionsMu.Lock()
		s.sessions[sessObj.SessionID] = sess
		s.sessionsMu.Unlock()
		if sessObj.IsClosed() {
			_ = s.CloseSession(sessObj.SessionID)
			return
		}
		if !s.startTransportTask(func() { s.acceptClientControlStream(sessObj, sess) }) {
			s.handleSessionClose(sessObj.SessionID)
		}
	}
}

const clientFrameTimeout = 15 * time.Second
const controlStreamTimeout = 10 * time.Second

func (s *Server) acceptClientControlStream(sessObj *session.Session, sess *webtransport.Session) {
	defer s.handleSessionClose(sessObj.SessionID)
	ctx, cancel := context.WithTimeout(sess.Context(), controlStreamTimeout)
	ctrl, err := sess.AcceptStream(ctx)
	cancel()
	if err != nil {
		logutil.Debugf("control stream accept failed (sess %d): %v", sessObj.SessionID, err)
		return
	}
	if !sessObj.AttachControlStream(ctrl) {
		ctrl.CancelRead(0)
		ctrl.CancelWrite(0)
		return
	}
	// Exactly one control stream owns reliable commands and responses. Extra
	// streams must never replace it or execute concurrent commands.
	if !s.startTransportTask(func() {
		extra, err := sess.AcceptStream(sess.Context())
		if err != nil {
			return
		}
		extra.CancelRead(0)
		extra.CancelWrite(0)
		s.handleSessionClose(sessObj.SessionID)
	}) {
		return
	}
	if !s.startTransportTask(func() { s.handleDatagrams(sessObj, sess) }) {
		return
	}
	s.handleControlStream(sessObj, ctrl)
}

func (s *Server) handleDatagrams(sessObj *session.Session, sess *webtransport.Session) {
	defer s.handleSessionClose(sessObj.SessionID)
	for {
		data, err := sess.ReceiveDatagram(sess.Context())
		if err != nil {
			return
		}
		if len(data) < 2 || len(data) > api.MaxClientPacketSize {
			return
		}
		s.worldHandler.HandlePacket(sessObj, data)
	}
}

// handleControlStream shares framing and deadlines across both transports.
func (s *Server) handleControlStream(sessObj *session.Session, ctrl io.ReadWriteCloser) {
	defer s.handleSessionClose(sessObj.SessionID)
	for {
		deadlineStream, hasDeadline := ctrl.(interface{ SetReadDeadline(time.Time) error })
		if hasDeadline {
			if err := deadlineStream.SetReadDeadline(time.Time{}); err != nil {
				return
			}
		}
		// Heartbeats may arrive over datagrams while the reliable stream is idle.
		// Session expiry owns idle time; a fixed frame deadline starts only once
		// the first byte arrives, so trickling a partial command cannot extend it.
		var first [1]byte
		if _, err := io.ReadFull(ctrl, first[:]); err != nil {
			return
		}
		if hasDeadline {
			if err := deadlineStream.SetReadDeadline(time.Now().Add(clientFrameTimeout)); err != nil {
				return
			}
		}
		payload, err := api.ReadClientFrame(io.MultiReader(bytes.NewReader(first[:]), ctrl))
		if err != nil {
			logutil.Debugf("control frame ended (sess %d): %v", sessObj.SessionID, err)
			return
		}
		s.worldHandler.HandlePacket(sessObj, payload)
	}
}

// SendStream writes data to a session's control stream.
func (s *Server) SendStream(sessionID int, data []byte) error {
	sessObj, ok := s.sessionManager.GetSession(sessionID)
	if !ok {
		return fmt.Errorf("session %d not found", sessionID)
	}
	return sessObj.WriteControlStream(data)
}

// SendDatagram fires a datagram packet to a client.
func (s *Server) SendDatagram(sessionID int, data []byte) error {
	s.sessionsMu.Lock()
	sess, ok := s.sessions[sessionID]
	s.sessionsMu.Unlock()
	if !ok {
		return fmt.Errorf("session %d not found", sessionID)
	}
	if err := sess.SendDatagram(data); err != nil {
		log.Printf("failed to send datagram: %v", err)
		return err
	}
	return nil
}

// CloseSession closes only the transport owned by this session. Called by
// Session.Close after the session manager releases its membership lock.
func (s *Server) CloseSession(sessionID int) error {
	s.sessionsMu.Lock()
	transport := s.sessions[sessionID]
	delete(s.sessions, sessionID)
	s.sessionsMu.Unlock()
	if transport != nil {
		return transport.CloseWithError(0, "session closed")
	}
	return nil
}

func (s *Server) handleSessionClose(sessionID int) {
	s.worldHandler.RemoveSession(sessionID)
}

func (s *Server) shutdownBudget() time.Duration {
	if s.gracePeriod > 0 {
		return s.gracePeriod
	}
	return 30 * time.Second
}

// StopServer uses the configured grace period for the caller's wait.
func (s *Server) StopServer() {
	budget := s.shutdownBudget()
	ctx, cancel := context.WithTimeout(context.Background(), budget)
	defer cancel()
	if err := s.StopServerContext(ctx); err != nil {
		log.Printf("Server shutdown: %v", err)
	}
}

// StopServerContext starts one owned drain and permits later callers to join it.
// On timeout storage stays open until HTTP handlers and world work have joined.
func (s *Server) StopServerContext(ctx context.Context) error {
	s.stopOnce.Do(func() {
		s.stopDone = make(chan struct{})
		go func() { s.drain(); close(s.stopDone) }()
	})
	select {
	case <-s.stopDone:
		return s.stopErr
	case <-ctx.Done():
		return fmt.Errorf("server drain unfinished: %w", ctx.Err())
	}
}

func (s *Server) drain() {
	s.draining.Store(true)
	s.ready.Store(false)
	s.lifecycleMu.Lock()
	s.stopping = true
	s.lifecycleMu.Unlock()
	if s.sessionManager != nil {
		s.sessionManager.Seal()
	}
	// Start world retirement before joining HTTP. It registers shutdown cleanup
	// before closing sessions, so racing transport callbacks retain save failures.
	worldDone := make(chan error, 1)
	go func() {
		var err error
		if s.worldHandler != nil {
			err = s.worldHandler.ShutdownContext(context.Background())
		}
		worldDone <- err
	}()
	wtClosed := make(chan error, 1)
	go func() {
		var err error
		s.closeQUICConnections()
		if s.wtServer != nil {
			err = s.wtServer.Close()
		}
		wtClosed <- err
	}()
	if s.httpCancel != nil {
		s.httpCancel()
	}
	if s.discordChat != nil {
		s.discordChat.Close()
	}
	// Shutdown joins ordinary HTTP handlers. Hijacked WebSockets are owned by
	// the session manager and retired by the concurrently draining world.
	// Persistence cleanup uses its own context and is joined below.
	if s.httpServer != nil {
		httpDrain, cancel := context.WithTimeout(context.Background(), s.shutdownBudget())
		err := s.httpServer.Shutdown(httpDrain)
		cancel()
		if err != nil {
			s.stopErr = errors.Join(s.stopErr, fmt.Errorf("HTTP shutdown: %w", err))
			// Close interrupts blocked socket/body reads; it does not join handlers.
			s.stopErr = errors.Join(s.stopErr, s.httpServer.Close())
		}
		<-s.httpDone
		// No handler can register after draining becomes true. Synchronize with
		// admissions already in progress before waiting on their owned work.
		s.httpHandlerMu.Lock()
		s.httpHandlerMu.Unlock()
		s.httpHandlers.Wait()
	}
	s.stopErr = errors.Join(s.stopErr, <-wtClosed)
	if s.udpConn != nil {
		_ = s.udpConn.Close()
	}
	if s.wtDone != nil {
		<-s.wtDone
	}
	s.transportMu.Lock()
	s.transportMu.Unlock()
	s.transportTasks.Wait()
	s.stopErr = errors.Join(s.stopErr, <-worldDone)
	if s.database != nil {
		s.stopErr = errors.Join(s.stopErr, s.database.Close())
	}
}

// serveHTTP owns the listener and serve completion for the server lifecycle.
// Startup calls it while holding lifecycleMu, after binding succeeds.
func (s *Server) serveHTTP(listener net.Listener, handler http.Handler) {
	httpContext, cancel := context.WithCancel(context.Background())
	s.httpCancel = cancel
	s.httpServer = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			s.httpHandlerMu.Lock()
			if s.draining.Load() {
				s.httpHandlerMu.Unlock()
				http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
				return
			}
			s.httpHandlers.Add(1)
			s.httpHandlerMu.Unlock()
			defer s.httpHandlers.Done()
			handler.ServeHTTP(w, r)
		}), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
		BaseContext: func(net.Listener) context.Context { return httpContext },
	}
	s.httpDone = make(chan struct{})
	go func() {
		defer close(s.httpDone)
		if err := s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.reportServeFailure(fmt.Errorf("HTTP serve: %w", err))
		}
	}()
}

// listenUDP binds to the given port.
func listenUDP(port int) (*net.UDPConn, int, error) {
	addr := fmt.Sprintf(":%d", port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, 0, err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, 0, err
	}
	conn.SetReadBuffer(4 * 1024 * 1024)
	conn.SetWriteBuffer(4 * 1024 * 1024)
	return conn, conn.LocalAddr().(*net.UDPAddr).Port, nil
}

// startHTTPServer serves HTTPS for other endpoints.
func (s *Server) startHTTPServer(tlsConf *tls.Config, certManager *cert.RotatingCertManager, port int, wtPort int) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ready", s.handleReadiness)
	mux.HandleFunc("/register", registerHandler)

	// WebSocket fallback for browsers without WebTransport (Safari, iOS)
	s.registerWSHandler(mux)

	// /api/hash returns the SHA-256 (base64) of the server certificate for WebTransport pinning.
	// When a RotatingCertManager is active, always read the live hash from it so clients
	// automatically get the new hash after a certificate rotation (no server restart needed).
	mux.Handle("/api/hash", corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var hashB64 string
		if certManager != nil {
			hashB64 = certManager.GetHash()
		} else {
			if len(tlsConf.Certificates) == 0 || len(tlsConf.Certificates[0].Certificate) == 0 {
				http.Error(w, "No certificate loaded", http.StatusInternalServerError)
				return
			}
			leafDER := tlsConf.Certificates[0].Certificate[0]
			sum := sha256.Sum256(leafDER)
			hashB64 = b64.StdEncoding.EncodeToString(sum[:])
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Alt-Svc", fmt.Sprintf(`h3=":%d"; ma=86400`, wtPort))
		w.Write([]byte(hashB64))
	})))

	mux.Handle("/api/online", corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logutil.Debugf("Received /api/online request from %s", r.RemoteAddr)
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Alt-Svc", fmt.Sprintf(`h3=":%d"; ma=86400`, wtPort))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Server is online"))
	})))

	mux.Handle("/api/playercount", corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logutil.Debugf("Received /api/playercount request from %s", r.RemoteAddr)
		count := session.GetActiveSessionCount()

		type response struct {
			Count int `json:"count"`
		}

		res := response{Count: count}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(res); err != nil {
			log.Printf("Error encoding JSON: %v", err)
			http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
			return
		}
	})))
	if s.discordChat != nil {
		mux.Handle("/api/discord/game-chat", s.discordChat.Handler())
	}
	s.registerOverworldOverviewRoute(mux)

	s.registerAdminRoutes(mux)

	// Tile authoring is a local-development surface. Production clients still
	// receive static tile artwork below, but cannot reach filesystem/database
	// mutation handlers even by constructing the HTTP requests directly.
	s.registerTileArtStudioRoutes(mux)

	// Phaser static assets (tile images and sprites)
	mux.Handle("/phaser/tiles/", corsMiddleware(http.StripPrefix("/phaser/tiles/", http.FileServer(http.Dir("../public/phaser/tile_images")))))
	mux.Handle("/phaser/sprites/", corsMiddleware(http.StripPrefix("/phaser/sprites/", http.FileServer(http.Dir("../public/phaser/sprites")))))

	// If a specific port is provided (e.g. from env var), we assume we're behind a proxy (Fly.io)
	// that handles TLS for us, so we listen on plain HTTP.
	if port > 0 {
		log.Printf("Starting plain HTTP server on TCP port %d (TLS terminated by proxy)", port)
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			return fmt.Errorf("listen HTTP: %w", err)
		}
		s.serveHTTP(listener, mux)
		return nil
	}

	// Local mode: listen on 443 with TLS
	listener, err := net.Listen("tcp", ":443")
	if err != nil {
		return fmt.Errorf("listen HTTPS: %w", err)
	}
	tlsListener := tls.NewListener(listener, tlsConf)
	log.Printf("Starting HTTPS server on TCP port 443 (Local TLS)")
	s.serveHTTP(tlsListener, mux)
	return nil
}

func (s *Server) registerAdminRoutes(mux *http.ServeMux) {
	register := func(path string, method string, handler http.HandlerFunc) {
		mux.Handle(path, adminAuthMiddleware(requireAdminMethod(method, handler)))
	}
	register("/api/admin/stats", http.MethodGet, handleAdminStats)
	register("/api/admin/growth", http.MethodGet, handleAdminGrowth)
	register("/api/admin/chats", http.MethodGet, handleAdminChats)
	register("/api/admin/users", http.MethodGet, handleAdminUsers)
	register("/api/admin/characters", http.MethodGet, handleAdminCharacters)
	register("/api/admin/set-gm", http.MethodPost, handleAdminSetGM)
	register("/api/admin/character/inventory", http.MethodGet, handleAdminGetCharacterInventory)
	register("/api/admin/logs", http.MethodGet, handleAdminLogs)
}

func (s *Server) registerOverworldOverviewRoute(mux *http.ServeMux) {
	service, err := overworldoverview.NewRuntimeService(db.GlobalWorldDB.DB, !s.debugMode)
	if err != nil {
		overworldoverview.SetDefault(nil)
		log.Printf("[OverworldOverview] Disabled: %v", err)
		return
	}
	overworldoverview.SetDefault(service)
	mux.Handle("/api/overworld/overview", corsMiddleware(service))
	log.Printf("[OverworldOverview] Serving %dx%d-tile chunks at %d pixels per tile",
		overworldoverview.ChunkTileSpan, overworldoverview.ChunkTileSpan, overworldoverview.PixelsPerTile)
}

func (s *Server) registerTileArtStudioRoutes(mux *http.ServeMux) {
	if !s.debugMode {
		return
	}
	mux.Handle("/api/tiles/replace", corsMiddleware(http.HandlerFunc(handleTileReplace)))
	mux.Handle("/api/tiles/stamp", corsMiddleware(http.HandlerFunc(handleStampCreate)))
	mux.Handle("/api/tiles/stamps", corsMiddleware(http.HandlerFunc(handleStampList)))
	mux.Handle("/api/tiles/animation", corsMiddleware(http.HandlerFunc(handleTileAnimationCreate)))
	mux.Handle("/api/tiles/animations", corsMiddleware(http.HandlerFunc(handleTileAnimationList)))
}

// registerHandler is used by internal services.
func registerHandler(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.RemoteAddr, "127.0.0.1:") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	w.Write([]byte("OK"))
}

// corsMiddleware enables CORS for HTTP endpoints.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Token")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
