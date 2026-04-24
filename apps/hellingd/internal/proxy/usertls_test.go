package proxy_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/pki"
	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/proxy"
)

type fakeTLSProvider struct {
	cert *tls.Certificate
	err  error
}

func (f *fakeTLSProvider) GetTLSCert(_ context.Context, _ string) (*tls.Certificate, error) {
	return f.cert, f.err
}

func userCertFromCA(t *testing.T, ca *pki.CA, username, userID string) *tls.Certificate {
	t.Helper()
	uc, err := ca.IssueUserCert(pki.UserCertRequest{Username: username, UserID: userID})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	pair, err := tls.X509KeyPair(uc.CertPEM, uc.PrivateKeyPEM)
	if err != nil {
		t.Fatalf("x509 keypair: %v", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}
	pair.Leaf = leaf
	return &pair
}

func readAll(t *testing.T, r io.Reader) string {
	t.Helper()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(b)
}

func TestProxy_UserTLSProvider_ForwardsCert(t *testing.T) {
	svc, token := newSvcWithAdmin(t)
	ca, err := pki.Bootstrap(nil)
	if err != nil {
		t.Fatal(err)
	}
	cert := userCertFromCA(t, ca, "alice", "u_alice")

	clientPool := x509.NewCertPool()
	clientPool.AddCert(ca.Cert)

	var sawSerial string
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			sawSerial = r.TLS.PeerCertificates[0].SerialNumber.Text(16)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	upstream.TLS = &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientPool,
		MinVersion: tls.VersionTLS12,
	}
	upstream.StartTLS()
	t.Cleanup(upstream.Close)

	provider := &fakeTLSProvider{cert: cert}
	cfg := &proxy.Config{
		IncusURL:           upstream.URL,
		InsecureSkipVerify: true,
		UserTLSProvider:    provider,
	}
	p, err := proxy.New(cfg, svc, nil)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/incus/", p.IncusHandler())
	edge := httptest.NewServer(mux)
	t.Cleanup(edge.Close)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet,
		edge.URL+"/api/incus/1.0/instances", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	wantSerial := cert.Leaf.SerialNumber.Text(16)
	if sawSerial != wantSerial {
		t.Fatalf("upstream saw serial %q, want %q", sawSerial, wantSerial)
	}
}

func TestProxy_UserTLSProvider_FallsBackOnNoCert(t *testing.T) {
	svc, token := newSvcWithAdmin(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"shared":true}`))
	}))
	t.Cleanup(upstream.Close)

	provider := &fakeTLSProvider{err: proxy.ErrNoUserCert}
	p, err := proxy.New(&proxy.Config{
		IncusURL:        upstream.URL,
		UserTLSProvider: provider,
	}, svc, nil)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/incus/", p.IncusHandler())
	edge := httptest.NewServer(mux)
	t.Cleanup(edge.Close)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet,
		edge.URL+"/api/incus/1.0/instances", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (fallback)", resp.StatusCode)
	}
	body := readAll(t, resp.Body)
	if !strings.Contains(body, `"shared":true`) {
		t.Fatalf("expected shared response, got %q", body)
	}
}

func TestProxy_UserTLSProvider_HardFailureReturns502(t *testing.T) {
	svc, token := newSvcWithAdmin(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	provider := &fakeTLSProvider{err: errors.New("db down")}
	p, _ := proxy.New(&proxy.Config{
		IncusURL:        upstream.URL,
		UserTLSProvider: provider,
	}, svc, nil)
	mux := http.NewServeMux()
	mux.Handle("/api/incus/", p.IncusHandler())
	edge := httptest.NewServer(mux)
	t.Cleanup(edge.Close)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet,
		edge.URL+"/api/incus/1.0/instances", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
}
