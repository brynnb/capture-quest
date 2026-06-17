package cert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	_ "embed"
	b64 "encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"capturequest/internal/config"

	"github.com/quic-go/quic-go/http3"
)

// certEntry holds a TLS certificate and its precomputed SHA-256 hash.
type certEntry struct {
	cert     tls.Certificate
	hash     [32]byte
	notAfter time.Time
}

// RotatingCertManager holds a self-renewing short-lived certificate.
// It satisfies the tls.Config.GetCertificate contract and keeps the
// /api/hash endpoint always returning the current cert's hash.
type RotatingCertManager struct {
	current atomic.Pointer[certEntry]
}

// NewRotatingCertManager creates a manager and starts the background renewal loop.
func NewRotatingCertManager() (*RotatingCertManager, error) {
	m := &RotatingCertManager{}
	if err := m.rotate(); err != nil {
		return nil, err
	}
	go m.renewLoop()
	return m, nil
}

// GetCertificate implements the tls.Config.GetCertificate callback.
func (m *RotatingCertManager) GetCertificate(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
	e := m.current.Load()
	return &e.cert, nil
}

// GetHash returns the SHA-256 (base64) of the current certificate for WebTransport pinning.
func (m *RotatingCertManager) GetHash() string {
	e := m.current.Load()
	return b64.StdEncoding.EncodeToString(e.hash[:])
}

// rotate generates a fresh 10-day certificate and atomically replaces the current one.
func (m *RotatingCertManager) rotate() error {
	now := time.Now()
	_, x509Bytes, priv, err := generateCert(now, now.Add(10*24*time.Hour))
	if err != nil {
		return fmt.Errorf("cert rotation failed: %w", err)
	}
	derBuf, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: x509Bytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: derBuf})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("X509KeyPair: %w", err)
	}
	leaf, err := x509.ParseCertificate(tlsCert.Certificate[0])
	if err != nil {
		return fmt.Errorf("parse leaf: %w", err)
	}
	tlsCert.Leaf = leaf

	e := &certEntry{
		cert:     tlsCert,
		hash:     sha256.Sum256(leaf.Raw),
		notAfter: leaf.NotAfter,
	}
	m.current.Store(e)
	log.Printf("Certificate rotated, valid until %s (hash: %s)", e.notAfter.Format(time.RFC3339), b64.StdEncoding.EncodeToString(e.hash[:]))
	return nil
}

// renewLoop wakes up daily and rotates the cert when fewer than 2 days remain.
func (m *RotatingCertManager) renewLoop() {
	for {
		time.Sleep(24 * time.Hour)
		e := m.current.Load()
		if time.Until(e.notAfter) < 2*24*time.Hour {
			if err := m.rotate(); err != nil {
				log.Printf("cert renewal error: %v", err)
			}
		}
	}
}

// GenerateCertAndStartServer generates a certificate and starts an HTTP server with the hash
func GenerateCertAndStartServer() ([]byte, []byte) {
	tlsConf, x509AsBytes, err := getTLSConf(time.Now(), time.Now().Add(10*24*time.Hour))
	if err != nil {
		log.Fatal(err)
	}
	cert := tlsConf.Certificates[0]
	hash := sha256.Sum256(cert.Leaf.Raw)
	hashPort := 7100
	if p := os.Getenv("HASH_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			hashPort = v
		}
	}
	fmt.Printf("Starting cert hash server on port %d\n", hashPort)
	go runHTTPServer(hashPort, hash)

	derBuf, _ := x509.MarshalECPrivateKey(cert.PrivateKey.(*ecdsa.PrivateKey))

	pem1 := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: x509AsBytes,
	})

	priv := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: derBuf,
	})

	return pem1, priv
}

// GenerateTLSConfig creates a tls.Config from certificate and key PEM data.
func GenerateTLSConfig(certPEM, keyPEM []byte) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{http3.NextProtoH3, "h2", "http/1.1"},
	}, nil
}

// LoadTLSConfig loads TLS config, preferring embedded key.pem if available, falling back to dynamic generation.
// The returned RotatingCertManager is non-nil only when the auto-rotating path is used; callers should use
// manager.GetHash() for the /api/hash endpoint so it always reflects the current cert.
func LoadTLSConfig() (*tls.Config, *RotatingCertManager, error) {
	serverConfig, err := config.Get()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config: %v", err)
	}

	// WebTransport's serverCertificateHashes only works with certificates
	// that have a validity period of 14 days or less. We always use the
	// RotatingCertManager which generates 10-day certs and auto-renews
	// them before expiry — no server restart needed.
	local := serverConfig.Local
	if local {
		// Local/self-signed mode: always use auto-rotating dynamic certs
		fmt.Println("Local mode: generating auto-rotating dynamic certificate for WebTransport")
		tlsConf, manager, err := LoadRotatingTLSConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create rotating TLS config: %v", err)
		}
		return tlsConf, manager, nil
	}

	// Production: try embedded key.pem first (e.g. from Let's Encrypt)
	tlsConf, err := loadEmbeddedTLSConfig()
	if err == nil {
		fmt.Println("Using embedded certificate")
		return tlsConf, nil, nil
	}
	fmt.Printf("Failed to load embedded certificate: %v\n", err)

	// Fallback to auto-rotating dynamic generation
	fmt.Println("Generating auto-rotating dynamic certificate")
	tlsConf, manager, err := LoadRotatingTLSConfig()
	if err != nil {
		return nil, nil, err
	}
	return tlsConf, manager, nil
}

// LoadRotatingTLSConfig returns a tls.Config backed by a RotatingCertManager.
// The returned config uses GetCertificate so new connections always get the
// current (non-expired) certificate without a server restart.
func LoadRotatingTLSConfig() (*tls.Config, *RotatingCertManager, error) {
	manager, err := NewRotatingCertManager()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create rotating cert manager: %w", err)
	}
	hashPort := 7100
	if p := os.Getenv("HASH_PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			hashPort = v
		}
	}
	fmt.Printf("Starting cert hash server on port %d\n", hashPort)
	go runHashManagerServer(hashPort, manager)

	return &tls.Config{
		MinVersion:     tls.VersionTLS13,
		GetCertificate: manager.GetCertificate,
		NextProtos:     []string{http3.NextProtoH3, "h2", "http/1.1"},
	}, manager, nil
}

// loadEmbeddedTLSConfig loads TLS config from embedded key.pem, supporting both single cert and PEM chain
func loadEmbeddedTLSConfig() (*tls.Config, error) {
	// Read the embedded file
	pemDataString, err := config.GetCert()
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded key.pem: %v", err)
	}
	pemData := []byte(pemDataString)
	// Parse all PEM blocks
	var certPEM, keyPEM []byte
	var certCount int
	rest := pemData
	for {
		block, next := pem.Decode(rest)
		if block == nil {
			if len(rest) > 0 {
				return nil, fmt.Errorf("invalid PEM block in key.pem, remaining data: %s", string(rest))
			}
			break
		}
		switch block.Type {
		case "CERTIFICATE":
			// Append to certPEM, supporting multiple certificates in a chain
			certPEM = append(certPEM, pem.EncodeToMemory(block)...)
			certCount++
		case "PRIVATE KEY", "RSA PRIVATE KEY", "EC PRIVATE KEY":
			// Only one key expected, use the last one if multiple (unlikely)
			keyPEM = pem.EncodeToMemory(block)
		default:
			return nil, fmt.Errorf("unexpected PEM block type: %s", block.Type)
		}
		rest = next
	}

	if certCount == 0 {
		return nil, fmt.Errorf("no CERTIFICATE block found in key.pem")
	}
	if len(keyPEM) == 0 {
		return nil, fmt.Errorf("no PRIVATE KEY block found in key.pem")
	}

	return GenerateTLSConfig(certPEM, keyPEM)
}

func runHTTPServer(port int, certHash [32]byte) {
	mux := http.NewServeMux()
	fmt.Printf("Starting hash server on port %d\n", port)
	mux.HandleFunc("/hash", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(b64.StdEncoding.EncodeToString(certHash[:])))
	})
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	if err != nil {
		log.Printf("hash server on port %d stopped: %v", port, err)
	}
}

// runHashManagerServer serves the current cert hash from a RotatingCertManager.
func runHashManagerServer(port int, m *RotatingCertManager) {
	mux := http.NewServeMux()
	mux.HandleFunc("/hash", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(m.GetHash()))
	})
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	if err != nil {
		log.Printf("rotating hash server on port %d stopped: %v", port, err)
	}
}

func getTLSConf(start, end time.Time) (*tls.Config, []byte, error) {
	cert, bytes, priv, err := generateCert(start, end)
	if err != nil {
		return nil, nil, err
	}

	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{cert.Raw},
			PrivateKey:  priv,
			Leaf:        cert,
		}},
	}, bytes, nil
}

func generateCert(start, end time.Time) (*x509.Certificate, []byte, *ecdsa.PrivateKey, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return nil, nil, nil, err
	}
	serial := int64(binary.BigEndian.Uint64(b))
	if serial < 0 {
		serial = -serial
	}
	certTempl := &x509.Certificate{
		SerialNumber:          big.NewInt(serial),
		Subject:               pkix.Name{},
		NotBefore:             start,
		NotAfter:              end,
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caPrivateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, certTempl, certTempl, &caPrivateKey.PublicKey, caPrivateKey)
	if err != nil {
		return nil, nil, nil, err
	}
	ca, err := x509.ParseCertificate(caBytes)
	if err != nil {
		return nil, nil, nil, err
	}
	return ca, caBytes, caPrivateKey, nil
}
