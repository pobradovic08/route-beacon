package tlsutil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// CertificateLoader manages a TLS certificate with automatic reloading
// when the certificate files change on disk.
type CertificateLoader struct {
	certPath string
	keyPath  string
	cert     *tls.Certificate
	mu       sync.RWMutex
	watcher  *fsnotify.Watcher
	done     chan struct{}
}

// NewCertificateLoader creates a new CertificateLoader that watches
// the given cert and key files for changes.
func NewCertificateLoader(certPath, keyPath string) (*CertificateLoader, error) {
	cl := &CertificateLoader{
		certPath: certPath,
		keyPath:  keyPath,
		done:     make(chan struct{}),
	}

	if err := cl.loadCertificate(); err != nil {
		return nil, fmt.Errorf("initial certificate load: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create file watcher: %w", err)
	}
	cl.watcher = watcher

	if err := watcher.Add(certPath); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("watch cert file: %w", err)
	}
	if err := watcher.Add(keyPath); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("watch key file: %w", err)
	}

	go cl.watchLoop()

	return cl, nil
}

func (cl *CertificateLoader) loadCertificate() error {
	cert, err := tls.LoadX509KeyPair(cl.certPath, cl.keyPath)
	if err != nil {
		return err
	}
	cl.mu.Lock()
	cl.cert = &cert
	cl.mu.Unlock()
	return nil
}

func (cl *CertificateLoader) watchLoop() {
	for {
		select {
		case event, ok := <-cl.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				slog.Info("certificate file changed, reloading", "file", event.Name)
				if err := cl.loadCertificate(); err != nil {
					slog.Error("failed to reload certificate", "error", err)
				} else {
					slog.Info("certificate reloaded successfully")
				}
			}
			if event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove) {
				// File was renamed/removed (common in atomic cert rotation).
				// Re-establish the watch on the same path and attempt reload.
				slog.Info("certificate file rotated, re-watching", "file", event.Name)
				cl.watcher.Remove(event.Name)
				time.Sleep(100 * time.Millisecond) // brief wait for new file to appear
				if err := cl.watcher.Add(event.Name); err != nil {
					slog.Warn("failed to re-watch certificate file after rotation",
						"file", event.Name, "error", err)
				}
				if err := cl.loadCertificate(); err != nil {
					slog.Warn("failed to reload certificate after rotation", "error", err)
				} else {
					slog.Info("certificate reloaded after rotation")
				}
			}
		case err, ok := <-cl.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("certificate watcher error", "error", err)
		case <-cl.done:
			return
		}
	}
}

// GetCertificate returns the current certificate. Suitable for use as
// tls.Config.GetCertificate callback.
func (cl *CertificateLoader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	return cl.cert, nil
}

// GetClientCertificate returns the current certificate. Suitable for use as
// tls.Config.GetClientCertificate callback.
func (cl *CertificateLoader) GetClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	return cl.cert, nil
}

// Close stops the file watcher.
func (cl *CertificateLoader) Close() error {
	close(cl.done)
	return cl.watcher.Close()
}

// LoadCAPool reads a CA certificate file and returns a CertPool.
func LoadCAPool(caPath string) (*x509.CertPool, error) {
	caCert, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read CA certificate: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate from %s", caPath)
	}
	return pool, nil
}

// NewServerTLSConfig creates a TLS configuration for the central gRPC server
// that requires and verifies client certificates (mTLS).
func NewServerTLSConfig(certLoader *CertificateLoader, caPool *x509.CertPool) *tls.Config {
	return &tls.Config{
		GetCertificate: certLoader.GetCertificate,
		ClientCAs:      caPool,
		ClientAuth:     tls.RequireAndVerifyClientCert,
		MinVersion:     tls.VersionTLS13,
	}
}

// NewClientTLSConfig creates a TLS configuration for the collector gRPC client
// that presents a client certificate for mutual authentication.
func NewClientTLSConfig(certLoader *CertificateLoader, caPool *x509.CertPool, serverName string) *tls.Config {
	return &tls.Config{
		GetClientCertificate: certLoader.GetClientCertificate,
		RootCAs:              caPool,
		ServerName:           serverName,
		MinVersion:           tls.VersionTLS13,
	}
}

// ExtractCollectorID extracts the collector ID from the CN of a verified
// peer certificate in a TLS connection state.
func ExtractCollectorID(state tls.ConnectionState) (string, error) {
	if len(state.VerifiedChains) == 0 || len(state.VerifiedChains[0]) == 0 {
		return "", fmt.Errorf("no verified peer certificate")
	}
	cn := state.VerifiedChains[0][0].Subject.CommonName
	if cn == "" {
		return "", fmt.Errorf("peer certificate has empty CN")
	}
	return cn, nil
}
