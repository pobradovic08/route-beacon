package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// generateSelfSignedCert creates a self-signed certificate and private key,
// returning them as PEM-encoded byte slices.
func generateSelfSignedCert(t *testing.T, cn string) (certPEM, keyPEM []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatalf("generate serial: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return certPEM, keyPEM
}

func TestLoadCAPool(t *testing.T) {
	t.Run("valid PEM file loads successfully", func(t *testing.T) {
		certPEM, _ := generateSelfSignedCert(t, "test-ca")
		dir := t.TempDir()
		caPath := filepath.Join(dir, "ca.pem")
		if err := os.WriteFile(caPath, certPEM, 0600); err != nil {
			t.Fatalf("write CA file: %v", err)
		}

		pool, err := LoadCAPool(caPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pool == nil {
			t.Fatal("expected non-nil cert pool")
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		_, err := LoadCAPool("/nonexistent/path/ca.pem")
		if err == nil {
			t.Fatal("expected error for nonexistent file, got nil")
		}
	})

	t.Run("empty file returns error", func(t *testing.T) {
		dir := t.TempDir()
		caPath := filepath.Join(dir, "empty.pem")
		if err := os.WriteFile(caPath, []byte{}, 0600); err != nil {
			t.Fatalf("write empty file: %v", err)
		}

		_, err := LoadCAPool(caPath)
		if err == nil {
			t.Fatal("expected error for empty PEM file, got nil")
		}
	})

	t.Run("invalid PEM content returns error", func(t *testing.T) {
		dir := t.TempDir()
		caPath := filepath.Join(dir, "invalid.pem")
		if err := os.WriteFile(caPath, []byte("not a valid PEM"), 0600); err != nil {
			t.Fatalf("write invalid file: %v", err)
		}

		_, err := LoadCAPool(caPath)
		if err == nil {
			t.Fatal("expected error for invalid PEM content, got nil")
		}
	})
}

func TestExtractCollectorID(t *testing.T) {
	t.Run("valid connection state with CN returns CN", func(t *testing.T) {
		certPEM, _ := generateSelfSignedCert(t, "collector-nyc-01")

		block, _ := pem.Decode(certPEM)
		if block == nil {
			t.Fatal("failed to decode PEM block")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatalf("parse certificate: %v", err)
		}

		state := tls.ConnectionState{
			VerifiedChains: [][]*x509.Certificate{
				{cert},
			},
		}

		id, err := ExtractCollectorID(state)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if id != "collector-nyc-01" {
			t.Errorf("expected 'collector-nyc-01', got %q", id)
		}
	})

	t.Run("empty verified chains returns error", func(t *testing.T) {
		state := tls.ConnectionState{
			VerifiedChains: nil,
		}

		_, err := ExtractCollectorID(state)
		if err == nil {
			t.Fatal("expected error for empty verified chains, got nil")
		}
	})

	t.Run("empty chain slice returns error", func(t *testing.T) {
		state := tls.ConnectionState{
			VerifiedChains: [][]*x509.Certificate{
				{},
			},
		}

		_, err := ExtractCollectorID(state)
		if err == nil {
			t.Fatal("expected error for empty chain slice, got nil")
		}
	})

	t.Run("empty CN returns error", func(t *testing.T) {
		// Generate cert with empty CN
		certPEM, _ := generateSelfSignedCert(t, "")

		block, _ := pem.Decode(certPEM)
		if block == nil {
			t.Fatal("failed to decode PEM block")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatalf("parse certificate: %v", err)
		}

		state := tls.ConnectionState{
			VerifiedChains: [][]*x509.Certificate{
				{cert},
			},
		}

		_, err = ExtractCollectorID(state)
		if err == nil {
			t.Fatal("expected error for empty CN, got nil")
		}
	})
}

func TestCertificateLoader(t *testing.T) {
	t.Run("create loader and get certificate", func(t *testing.T) {
		dir := t.TempDir()
		certPEM, keyPEM := generateSelfSignedCert(t, "test-server")

		certPath := filepath.Join(dir, "cert.pem")
		keyPath := filepath.Join(dir, "key.pem")
		if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
			t.Fatalf("write cert: %v", err)
		}
		if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
			t.Fatalf("write key: %v", err)
		}

		loader, err := NewCertificateLoader(certPath, keyPath)
		if err != nil {
			t.Fatalf("create loader: %v", err)
		}
		defer loader.Close()

		cert, err := loader.GetCertificate(nil)
		if err != nil {
			t.Fatalf("get certificate: %v", err)
		}
		if cert == nil {
			t.Fatal("expected non-nil certificate")
		}
	})

	t.Run("GetClientCertificate returns non-nil", func(t *testing.T) {
		dir := t.TempDir()
		certPEM, keyPEM := generateSelfSignedCert(t, "test-client")

		certPath := filepath.Join(dir, "cert.pem")
		keyPath := filepath.Join(dir, "key.pem")
		if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
			t.Fatalf("write cert: %v", err)
		}
		if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
			t.Fatalf("write key: %v", err)
		}

		loader, err := NewCertificateLoader(certPath, keyPath)
		if err != nil {
			t.Fatalf("create loader: %v", err)
		}
		defer loader.Close()

		cert, err := loader.GetClientCertificate(nil)
		if err != nil {
			t.Fatalf("get client certificate: %v", err)
		}
		if cert == nil {
			t.Fatal("expected non-nil certificate")
		}
	})

	t.Run("invalid cert path returns error", func(t *testing.T) {
		_, err := NewCertificateLoader("/nonexistent/cert.pem", "/nonexistent/key.pem")
		if err == nil {
			t.Fatal("expected error for nonexistent cert files, got nil")
		}
	})

	t.Run("mismatched cert and key returns error", func(t *testing.T) {
		dir := t.TempDir()
		certPEM1, _ := generateSelfSignedCert(t, "cert1")
		_, keyPEM2 := generateSelfSignedCert(t, "cert2")

		certPath := filepath.Join(dir, "cert.pem")
		keyPath := filepath.Join(dir, "key.pem")
		if err := os.WriteFile(certPath, certPEM1, 0600); err != nil {
			t.Fatalf("write cert: %v", err)
		}
		if err := os.WriteFile(keyPath, keyPEM2, 0600); err != nil {
			t.Fatalf("write key: %v", err)
		}

		_, err := NewCertificateLoader(certPath, keyPath)
		if err == nil {
			t.Fatal("expected error for mismatched cert/key, got nil")
		}
	})

	t.Run("reload on cert file change", func(t *testing.T) {
		dir := t.TempDir()
		certPEM1, keyPEM1 := generateSelfSignedCert(t, "original-cn")

		certPath := filepath.Join(dir, "cert.pem")
		keyPath := filepath.Join(dir, "key.pem")
		if err := os.WriteFile(certPath, certPEM1, 0600); err != nil {
			t.Fatalf("write cert: %v", err)
		}
		if err := os.WriteFile(keyPath, keyPEM1, 0600); err != nil {
			t.Fatalf("write key: %v", err)
		}

		loader, err := NewCertificateLoader(certPath, keyPath)
		if err != nil {
			t.Fatalf("create loader: %v", err)
		}
		defer loader.Close()

		// Verify initial certificate is loaded
		cert1, err := loader.GetCertificate(nil)
		if err != nil {
			t.Fatalf("get initial certificate: %v", err)
		}
		if cert1 == nil {
			t.Fatal("expected non-nil initial certificate")
		}

		// Parse the initial cert to get its serial for comparison
		initialParsed, err := x509.ParseCertificate(cert1.Certificate[0])
		if err != nil {
			t.Fatalf("parse initial certificate: %v", err)
		}
		initialSerial := initialParsed.SerialNumber

		// Generate a new certificate and write it to the same paths
		certPEM2, keyPEM2 := generateSelfSignedCert(t, "reloaded-cn")
		if err := os.WriteFile(certPath, certPEM2, 0600); err != nil {
			t.Fatalf("write new cert: %v", err)
		}
		if err := os.WriteFile(keyPath, keyPEM2, 0600); err != nil {
			t.Fatalf("write new key: %v", err)
		}

		// Wait for the file watcher to detect the change and reload.
		// The fsnotify watcher should detect the write event.
		var cert2 *tls.Certificate
		var reloaded bool
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			cert2, err = loader.GetCertificate(nil)
			if err != nil {
				t.Fatalf("get reloaded certificate: %v", err)
			}
			if cert2 == nil {
				continue
			}
			parsed, err := x509.ParseCertificate(cert2.Certificate[0])
			if err != nil {
				t.Fatalf("parse reloaded certificate: %v", err)
			}
			if parsed.SerialNumber.Cmp(initialSerial) != 0 {
				reloaded = true
				// Also verify the CN changed
				if parsed.Subject.CommonName != "reloaded-cn" {
					t.Errorf("expected CN 'reloaded-cn', got %q", parsed.Subject.CommonName)
				}
				break
			}
		}

		if !reloaded {
			t.Error("certificate was not reloaded within 2 seconds after file change")
		}
	})
}

func TestNewServerTLSConfig(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM := generateSelfSignedCert(t, "server")

	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	loader, err := NewCertificateLoader(certPath, keyPath)
	if err != nil {
		t.Fatalf("create loader: %v", err)
	}
	defer loader.Close()

	caPath := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caPath, certPEM, 0600); err != nil {
		t.Fatalf("write CA: %v", err)
	}
	pool, err := LoadCAPool(caPath)
	if err != nil {
		t.Fatalf("load CA pool: %v", err)
	}

	cfg := NewServerTLSConfig(loader, pool)
	if cfg == nil {
		t.Fatal("expected non-nil TLS config")
	}
	if cfg.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Error("expected RequireAndVerifyClientCert")
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Error("expected minimum TLS 1.3")
	}
	if cfg.GetCertificate == nil {
		t.Error("expected GetCertificate callback to be set")
	}
	if cfg.ClientCAs == nil {
		t.Error("expected ClientCAs to be set")
	}
}

func TestNewClientTLSConfig(t *testing.T) {
	dir := t.TempDir()
	certPEM, keyPEM := generateSelfSignedCert(t, "client")

	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	loader, err := NewCertificateLoader(certPath, keyPath)
	if err != nil {
		t.Fatalf("create loader: %v", err)
	}
	defer loader.Close()

	caPath := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caPath, certPEM, 0600); err != nil {
		t.Fatalf("write CA: %v", err)
	}
	pool, err := LoadCAPool(caPath)
	if err != nil {
		t.Fatalf("load CA pool: %v", err)
	}

	cfg := NewClientTLSConfig(loader, pool, "server.example.com")
	if cfg == nil {
		t.Fatal("expected non-nil TLS config")
	}
	if cfg.ServerName != "server.example.com" {
		t.Errorf("expected ServerName 'server.example.com', got %q", cfg.ServerName)
	}
	if cfg.MinVersion != tls.VersionTLS13 {
		t.Error("expected minimum TLS 1.3")
	}
	if cfg.GetClientCertificate == nil {
		t.Error("expected GetClientCertificate callback to be set")
	}
	if cfg.RootCAs == nil {
		t.Error("expected RootCAs to be set")
	}
}
