package proxy

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
	"time"
)

// UserTLSProvider returns a per-user TLS client certificate for outbound
// Incus calls per ADR-024. Implementations look up the active row from
// authrepo, decrypt the age-encrypted PEM blobs, and assemble a ready-to-use
// tls.Certificate. Returning ErrNoUserCert is a soft signal: the proxy
// falls back to the shared admin cert (if configured) so dev environments
// without per-user provisioning keep working.
type UserTLSProvider interface {
	GetTLSCert(ctx context.Context, userID string) (*tls.Certificate, error)
}

// ErrNoUserCert signals "no per-user cert is on file"; callers should fall
// back to the shared admin cert.
var ErrNoUserCert = errors.New("proxy: no user cert")

type userTLSKey struct{}

// withUserID stores the resolved user id on the request context so the
// downstream Incus dispatcher can pick the right cert.
func withUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userTLSKey{}, userID)
}

// userIDFromContext is the inverse of withUserID. Returns "" when missing.
func userIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userTLSKey{}).(string)
	return v
}

// userTransportCache memoises per-user *http.Transport instances so we keep
// connection-pooling benefits across requests for the same user. Entries
// are evicted on certificate rotation when serial changes.
type userTransportCache struct {
	mu       sync.Mutex
	entries  map[string]*userTransportEntry
	rootCAs  *x509.CertPool
	insecure bool
}

type userTransportEntry struct {
	transport *http.Transport
	serial    string
}

func newUserTransportCache(rootCAs *x509.CertPool, insecure bool) *userTransportCache {
	return &userTransportCache{
		entries:  make(map[string]*userTransportEntry),
		rootCAs:  rootCAs,
		insecure: insecure,
	}
}

// transportFor returns a *http.Transport bound to cert for the given user.
// Reuses a cached entry when the serial matches; otherwise rebuilds.
func (c *userTransportCache) transportFor(userID string, cert *tls.Certificate) *http.Transport {
	c.mu.Lock()
	defer c.mu.Unlock()
	serial := certSerial(cert)
	if entry, ok := c.entries[userID]; ok && entry.serial == serial {
		return entry.transport
	}
	t := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			Certificates:       []tls.Certificate{*cert},
			RootCAs:            c.rootCAs,
			InsecureSkipVerify: c.insecure, //nolint:gosec // dev-only switch
		},
		MaxIdleConns:        4,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     5 * time.Minute,
	}
	c.entries[userID] = &userTransportEntry{transport: t, serial: serial}
	return t
}

func certSerial(c *tls.Certificate) string {
	if c == nil || c.Leaf == nil || c.Leaf.SerialNumber == nil {
		return ""
	}
	return c.Leaf.SerialNumber.Text(16)
}

// buildIncusUserHandler returns a handler that selects a per-user transport
// for each request and falls back to the shared transport when the provider
// signals no cert. The shared transport is what buildIncusHandler returns;
// we keep it as the fallback so legacy single-cert setups keep working.
func buildIncusUserHandler(cfg *Config, shared http.Handler, provider UserTLSProvider) (http.Handler, error) {
	target, err := url.Parse(cfg.IncusURL)
	if err != nil {
		return nil, err
	}
	rootCAs, err := loadIncusRootCAs(cfg.IncusCAPath)
	if err != nil && !errors.Is(err, errNoIncusCA) {
		return nil, err
	}
	cache := newUserTransportCache(rootCAs, cfg.InsecureSkipVerify)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := userIDFromContext(r.Context())
		if userID == "" {
			shared.ServeHTTP(w, r)
			return
		}
		cert, err := provider.GetTLSCert(r.Context(), userID)
		if err != nil {
			if errors.Is(err, ErrNoUserCert) {
				shared.ServeHTTP(w, r)
				return
			}
			writeErr(w, "incus", http.StatusBadGateway, "INCUS_USER_CERT_LOOKUP_FAILED: "+err.Error())
			return
		}
		t := cache.transportFor(userID, cert)
		rp := httputil.NewSingleHostReverseProxy(target)
		rp.Transport = t
		rp.ErrorHandler = incusErrorHandler
		rp.ModifyResponse = tagIncusResponse
		rp.ServeHTTP(w, r)
	}), nil
}

// errNoIncusCA is returned by loadIncusRootCAs when the operator did not
// configure HELLING_INCUS_CA. Callers treat this as "use system roots".
var errNoIncusCA = errors.New("proxy: no incus ca configured")

// loadIncusRootCAs centralizes the optional Incus CA bundle load so both
// shared- and per-user-cert paths resolve trust the same way.
func loadIncusRootCAs(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, errNoIncusCA
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	pem, err := os.ReadFile(path) //nolint:gosec // operator-controlled host path
	if err != nil {
		return nil, fmt.Errorf("proxy: read incus ca: %w", err)
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, errors.New("proxy: incus ca pem invalid")
	}
	return pool, nil
}
