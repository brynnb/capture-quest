package server

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	b64 "encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"capturequest/internal/api/opcodes"
	"capturequest/internal/cache"
	"capturequest/internal/cert"
	"capturequest/internal/config"
	"capturequest/internal/db"
	"capturequest/internal/logutil"
	"capturequest/internal/session"
	"capturequest/internal/world"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/webtransport-go"
)

// Server hosts a WebTransport-based world server with both datagrams and a single control stream per session.
type Server struct {
	wtServer       *webtransport.Server
	worldHandler   *world.WorldHandler
	sessionManager *session.SessionManager
	sessions       map[int]*webtransport.Session
	sessionsMu     sync.Mutex // Protects sessions map
	udpConn        *net.UDPConn
	gracePeriod    time.Duration
	debugMode      bool
}

// NewServer constructs a new Server.
func NewServer(dsn string, gracePeriod time.Duration, debugMode bool) (*Server, error) {
	sessionManager := session.NewSessionManager()
	session.InitSessionManager(sessionManager)
	worldHandler := world.NewWorldHandler(sessionManager)

	if err := cache.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}

	return &Server{
		worldHandler:   worldHandler,
		sessionManager: sessionManager,
		sessions:       make(map[int]*webtransport.Session),
		gracePeriod:    gracePeriod,
		debugMode:      debugMode,
	}, nil
}

// StartServer configures TLS, QUIC, HTTP, and begins serving WebTransport.
func (s *Server) StartServer() {
	InitLogging()
	// TLS
	tlsConf, certManager, err := cert.LoadTLSConfig()
	if err != nil {
		log.Printf("failed to load TLS config: %v", err)
		return
	}

	// Bind UDP for WebTransport. Local dev can override WT_PORT when the
	// default port is already occupied by another running server.
	wtPort := envPort("WT_PORT", 4433)
	udpConn, port, err := listenUDP(wtPort)
	if err != nil {
		log.Printf("UDP listen error on port %d: %v", wtPort, err)
		return
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
	cfg, _ := config.Get()
	go s.startHTTPServer(tlsConf, certManager, cfg.HTTPPort, port)

	// Serve WebTransport on the pre-bound UDP socket
	go func() {
		log.Printf("Starting WebTransport server on UDP port %d (HTTP/3)", port)
		if err := s.wtServer.Serve(udpConn); err != nil {
			log.Printf("WebTransport server failed: %v", err)
		}
	}()
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
	var nextID int
	return func(rw http.ResponseWriter, r *http.Request) {
		logutil.Debugf("Received /cq WebTransport request from %s", r.RemoteAddr)
		logutil.Debugf("Request method: %s, URL: %s", r.Method, r.URL.String())
		sess, err := s.wtServer.Upgrade(rw, r)
		if err != nil {
			log.Printf("Upgrade error: %v", err)
			return
		}

		clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		params := r.URL.Query()

		// Try reconnect
		var sessObj *session.Session
		if sidStr := params.Get("sid"); sidStr != "0" {
			if sid, e := strconv.Atoi(sidStr); e == nil {
				if existing, e2 := s.sessionManager.GetValidSession(sid, clientIP); e2 == nil {
					log.Printf("Reconnecting session %d from %s", sid, clientIP)
					sessObj = existing
					existing.Messenger = s
					existing.SendJSON(map[string]interface{}{}, opcodes.Reconnect)
				}
			}
		}

		// New session
		if sessObj == nil {
			nextID++
			sid := nextID
			s.sessionsMu.Lock()
			s.sessions[sid] = sess
			s.sessionsMu.Unlock()

			// Open a single control stream (bidi)
			ctrl, e := sess.OpenStream()
			if e != nil {
				log.Printf("Failed to open control stream: %v", e)
				sess.CloseWithError(400, "ctrl stream failed")
				return
			}

			log.Printf("Accepted new session %d", sid)
			sessObj = s.sessionManager.CreateSession(s, sid, clientIP, ctrl)

			// Send an initial empty frame to ensure the client sees the stream
			// (WebTransport may not notify JS until data is sent)
			initialFrame := make([]byte, 6)                     // 4-byte length (0) + 2-byte opcode (0)
			binary.LittleEndian.PutUint32(initialFrame[0:4], 2) // length = 2 (just opcode)
			binary.LittleEndian.PutUint16(initialFrame[4:6], 0) // opcode 0 = padding/noop
			if _, err := ctrl.Write(initialFrame); err != nil {
				log.Printf("Failed to send initial frame: %v", err)
			}

			// Start control stream reader
			go s.handleControlStream(sessObj, ctrl, sid, clientIP)
		}

		go s.acceptClientControlStreams(sessObj, sess, sessObj.SessionID, clientIP)

		// Start datagram reader
		go s.handleDatagrams(sessObj, sess)
	}
}

func (s *Server) acceptClientControlStreams(sessObj *session.Session, sess *webtransport.Session, sid int, clientIP string) {
	for {
		ctrl, err := sess.AcceptStream(context.Background())
		if err != nil {
			logutil.Debugf("client-opened control stream accept closed (sess %d): %v", sid, err)
			return
		}
		log.Printf("Accepted client-opened control stream for session %d", sid)
		sessObj.ControlStream = ctrl
		go s.handleControlStream(sessObj, ctrl, sid, clientIP)
	}
}

// handleDatagrams reads incoming datagrams forever.
func (s *Server) handleDatagrams(sessObj *session.Session, sess *webtransport.Session) {
	ctx := context.Background()
	for {
		data, err := sess.ReceiveDatagram(ctx)
		if err != nil {
			logutil.Debugf("datagram recv closed (sess %d): %v", sessObj.SessionID, err)
			s.handleSessionClose(sessObj.SessionID)
			return
		}
		s.worldHandler.HandlePacket(sessObj, data)
	}
}

// handleControlStream parses length-prefixed frames on the single bidi stream.
func (s *Server) handleControlStream(
	sessObj *session.Session,
	ctrl io.ReadWriteCloser,
	sid int,
	_ string,
) {
	defer ctrl.Close()
	for {
		// read length prefix
		var lenBuf [4]byte
		if _, err := io.ReadFull(ctrl, lenBuf[:]); err != nil {
			logutil.Debugf("ctrl read len error (sess %d): %v", sid, err)
			s.handleSessionClose(sid)
			return
		}
		n := binary.LittleEndian.Uint32(lenBuf[:])

		// read payload
		payload := make([]byte, n)
		if _, err := io.ReadFull(ctrl, payload); err != nil {
			logutil.Debugf("ctrl read payload error (sess %d): %v", sid, err)
			s.handleSessionClose(sid)
			return
		}

		// Handle JSON control stream messages
		s.worldHandler.HandlePacket(sessObj, payload)
		logutil.Debugf("sess %d control (JSON) -> %d bytes", sid, len(payload))
	}
}

// SendStream writes data to a session's control stream.
func (s *Server) SendStream(sessionID int, data []byte) error {
	sessObj, ok := s.sessionManager.GetSession(sessionID)
	if !ok {
		return fmt.Errorf("session %d not found", sessionID)
	}
	_, err := sessObj.ControlStream.Write(data)
	return err
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

// handleSessionClose schedules removal after gracePeriod.
func (s *Server) handleSessionClose(sessionID int) {
	// Remove the WebTransport session from our map to prevent memory leak
	s.sessionsMu.Lock()
	delete(s.sessions, sessionID)
	s.sessionsMu.Unlock()
	s.worldHandler.RemoveSession(sessionID)
	logutil.Debugf("Cleaned up session %d", sessionID)
}

// StopServer tears down all listeners and connections.
func (s *Server) StopServer() {
	if s.wtServer != nil {
		s.wtServer.Close()
	}
	if s.udpConn != nil {
		s.udpConn.Close()
	}
	s.worldHandler.Shutdown()
	if db.GlobalWorldDB != nil {
		db.GlobalWorldDB.DB.Close()
	}
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
func (s *Server) startHTTPServer(tlsConf *tls.Config, certManager *cert.RotatingCertManager, port int, wtPort int) {
	mux := http.NewServeMux()
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

	// Admin Dashboard API
	mux.Handle("/api/admin/stats", corsMiddleware(adminAuthMiddleware(handleAdminStats)))
	mux.Handle("/api/admin/users", corsMiddleware(adminAuthMiddleware(handleAdminUsers)))
	mux.Handle("/api/admin/characters", corsMiddleware(adminAuthMiddleware(handleAdminCharacters)))
	mux.Handle("/api/admin/set-gm", corsMiddleware(adminAuthMiddleware(handleAdminSetGM)))
	mux.Handle("/api/admin/db/tables", corsMiddleware(adminAuthMiddleware(handleAdminDBTables)))
	mux.Handle("/api/admin/db/query", corsMiddleware(adminAuthMiddleware(handleAdminDBQuery)))
	mux.Handle("/api/admin/character/inventory", corsMiddleware(adminAuthMiddleware(handleAdminGetCharacterInventory)))
	mux.Handle("/api/admin/logs", corsMiddleware(adminAuthMiddleware(handleAdminLogs)))

	// Tile Art Studio API
	mux.Handle("/api/tiles/replace", corsMiddleware(http.HandlerFunc(handleTileReplace)))
	mux.Handle("/api/tiles/stamp", corsMiddleware(http.HandlerFunc(handleStampCreate)))
	mux.Handle("/api/tiles/stamps", corsMiddleware(http.HandlerFunc(handleStampList)))
	mux.Handle("/api/tiles/animation", corsMiddleware(http.HandlerFunc(handleTileAnimationCreate)))
	mux.Handle("/api/tiles/animations", corsMiddleware(http.HandlerFunc(handleTileAnimationList)))

	// Phaser static assets (tile images and sprites)
	mux.Handle("/phaser/tiles/", corsMiddleware(http.StripPrefix("/phaser/tiles/", http.FileServer(http.Dir("../public/phaser/tile_images")))))
	mux.Handle("/phaser/sprites/", corsMiddleware(http.StripPrefix("/phaser/sprites/", http.FileServer(http.Dir("../public/phaser/sprites")))))

	// If a specific port is provided (e.g. from env var), we assume we're behind a proxy (Fly.io)
	// that handles TLS for us, so we listen on plain HTTP.
	if port > 0 {
		log.Printf("Starting plain HTTP server on TCP port %d (TLS terminated by proxy)", port)
		go http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
		return
	}

	// Local mode: listen on 443 with TLS
	listener, err := net.Listen("tcp", ":443")
	if err != nil {
		log.Printf("HTTPS listen error on 443: %v", err)
		return
	}
	tlsListener := tls.NewListener(listener, tlsConf)
	log.Printf("Starting HTTPS server on TCP port 443 (Local TLS)")
	go http.Serve(tlsListener, mux)
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
