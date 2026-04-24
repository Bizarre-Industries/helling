// Package main starts the hellingd daemon process.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	hellingapi "github.com/Bizarre-Industries/Helling/apps/hellingd/api"
	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/auth"
	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/db"
	httpserver "github.com/Bizarre-Industries/Helling/apps/hellingd/internal/http"
	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/pki"
	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/proxy"
	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/repo/authrepo"
)

const (
	defaultAddr = ":8080"
	defaultDSN  = "file:/var/lib/helling/helling.db?cache=shared"

	accessTokenTTL    = 15 * time.Minute
	refreshTokenTTL   = 7 * 24 * time.Hour
	sessionInactivity = 30 * time.Minute
	jwtIssuer         = "hellingd"
	jwtKeyPathEnvVar  = "HELLING_JWT_PRIVATE_KEY_PATH"

	// caDirEnvVar overrides the on-host directory containing ca-identity,
	// ca.key.age, and ca.crt. When unset, hellingd skips CA bootstrap so dev
	// runs do not write to /etc/helling.
	caDirEnvVar = "HELLING_CA_DIR"

	// certRenewalInterval is how often the renewal worker scans
	// user_certificates for rows nearing the 60-day threshold (spec §4.1).
	// Hourly is well below the renewal granularity (10-day grace) so we
	// catch any clock skew or restart-induced gap.
	certRenewalInterval = 1 * time.Hour
)

var defaultServe = func(server *http.Server) error {
	return server.ListenAndServe()
}

var (
	exitFunc           = os.Exit
	stderr   io.Writer = os.Stderr
	openDB             = db.Open
	osArgs             = os.Args
)

func newLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, nil))
}

// runConfig holds parsed CLI flags, kept small so tests can synthesize one
// without invoking flag.Parse directly.
type runConfig struct {
	addr        string
	dsn         string
	migrateOnly bool
}

func parseFlags(args []string, errOut io.Writer) (runConfig, error) {
	fs := flag.NewFlagSet("hellingd", flag.ContinueOnError)
	fs.SetOutput(errOut)

	cfg := runConfig{}
	fs.StringVar(&cfg.addr, "addr", defaultAddr, "listen address")
	fs.StringVar(&cfg.dsn, "db", defaultDSN, "SQLite DSN (modernc.org/sqlite)")
	fs.BoolVar(&cfg.migrateOnly, "migrate-only", false, "apply migrations and exit")

	if err := fs.Parse(args); err != nil {
		return runConfig{}, err
	}
	return cfg, nil
}

func run(logger *slog.Logger, cfg runConfig, serve func(*http.Server) error) int {
	ctx := context.Background()

	pool, err := openDB(ctx, cfg.dsn)
	if err != nil {
		logger.Error("open db", slog.Any("err", err))
		return 1
	}
	defer func() { _ = pool.Close() }()

	logger.Info("db ready",
		slog.String("dsn", cfg.dsn),
		slog.Bool("migrate_only", cfg.migrateOnly),
	)

	if cfg.migrateOnly {
		return 0
	}

	authSvc, err := buildAuthService(logger, pool)
	if err != nil {
		logger.Error("build auth service", slog.Any("err", err))
		return 1
	}

	ca, identity, err := ensureInternalCAWithIdentity(logger)
	if err != nil {
		logger.Error("ensure internal ca", slog.Any("err", err))
		return 1
	}

	var (
		issuer  hellingapi.CertIssuer
		userTLS proxy.UserTLSProvider
	)
	if ca != nil {
		concreteIssuer := &pki.Issuer{CA: ca, Identity: identity, Repo: authSvc.Repo()}
		issuer = concreteIssuer
		userTLS = &pkiTLSAdapter{repo: authSvc.Repo(), identity: identity}
		renewer := &pki.Renewer{Issuer: concreteIssuer, Repo: authSvc.Repo(), Logger: logger}
		go renewer.Run(ctx, certRenewalInterval)
	}

	proxyDeps, err := buildProxyDeps(logger, authSvc, userTLS)
	if err != nil {
		logger.Error("build proxy", slog.Any("err", err))
		return 1
	}

	mux := httpserver.NewMuxWith(hellingapi.Deps{
		Auth:        authSvc,
		IncusProxy:  proxyDeps.incus,
		PodmanProxy: proxyDeps.podman,
		CertIssuer:  issuer,
	})

	server := &http.Server{
		Addr:              cfg.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	logger.Info("hellingd listening", slog.String("addr", server.Addr))
	if err := serve(server); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("hellingd stopped", slog.Any("err", err))
		return 1
	}

	return 0
}

// proxyHandlers bundles the optional Incus/Podman reverse-proxy handlers.
type proxyHandlers struct {
	incus  http.Handler
	podman http.Handler
}

// buildProxyDeps wires the Incus + Podman proxy handlers from environment
// variables (see apps/hellingd/internal/proxy/proxy.go). Missing envs leave
// the matching route unmounted so hellingd runs fine in dev without Incus or
// Podman installed.
func buildProxyDeps(logger *slog.Logger, authSvc *auth.Service, userTLS proxy.UserTLSProvider) (proxyHandlers, error) {
	cfg := proxy.ConfigFromEnv()
	if cfg.IncusURL == "" && cfg.PodmanSocket == "" {
		logger.Info("proxy disabled (HELLING_INCUS_URL and HELLING_PODMAN_SOCKET unset)")
		return proxyHandlers{}, nil
	}
	cfg.UserTLSProvider = userTLS
	p, err := proxy.New(&cfg, authSvc, logger)
	if err != nil {
		return proxyHandlers{}, err
	}
	out := proxyHandlers{}
	if cfg.IncusURL != "" {
		out.incus = p.IncusHandler()
	}
	if cfg.PodmanSocket != "" {
		out.podman = p.PodmanHandler()
	}
	logger.Info("proxy enabled",
		slog.Bool("incus", cfg.IncusURL != ""),
		slog.Bool("podman", cfg.PodmanSocket != ""),
	)
	return out, nil
}

// pkiTLSAdapter bridges authrepo + pki into proxy.UserTLSProvider so the
// reverse proxy can present per-user mTLS certs to Incus (ADR-024 §6).
// Returns proxy.ErrNoUserCert when no active row exists or when the cert
// is past its grace window, signaling the proxy to fall back to the
// shared admin cert.
type pkiTLSAdapter struct {
	repo     *authrepo.Repo
	identity string
}

func (a *pkiTLSAdapter) GetTLSCert(ctx context.Context, userID string) (*tls.Certificate, error) {
	if a == nil || a.repo == nil {
		return nil, errors.New("pkiTLSAdapter: not configured")
	}
	if a.identity == "" {
		return nil, errors.New("pkiTLSAdapter: missing age identity")
	}
	row, err := a.repo.GetActiveUserCertificate(ctx, userID)
	if err != nil {
		if errors.Is(err, authrepo.ErrNotFound) {
			return nil, proxy.ErrNoUserCert
		}
		return nil, fmt.Errorf("pkiTLSAdapter: lookup: %w", err)
	}
	if pki.Expired(time.Unix(row.ExpiresAt, 0), time.Now()) {
		return nil, proxy.ErrNoUserCert
	}
	certPEM, err := pki.DecryptWithIdentity(a.identity, row.CertPEM)
	if err != nil {
		return nil, fmt.Errorf("pkiTLSAdapter: decrypt cert: %w", err)
	}
	keyPEM, err := pki.DecryptWithIdentity(a.identity, row.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("pkiTLSAdapter: decrypt key: %w", err)
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("pkiTLSAdapter: x509 keypair: %w", err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("pkiTLSAdapter: parse leaf: %w", err)
	}
	pair.Leaf = leaf
	return &pair, nil
}

// ensureInternalCAWithIdentity bootstraps or loads the Helling internal CA
// per ADR-024 when HELLING_CA_DIR is set, and returns both the live CA and
// the on-host age identity needed to encrypt persisted user-cert blobs.
// Dev runs without HELLING_CA_DIR get (nil, "", nil) so callers treat PKI
// as off without writing anything to /etc/helling.
func ensureInternalCAWithIdentity(logger *slog.Logger) (*pki.CA, string, error) {
	dir := os.Getenv(caDirEnvVar)
	if dir == "" {
		logger.Info("internal ca disabled (HELLING_CA_DIR unset)")
		return nil, "", nil
	}
	paths := pki.NewTestPaths(dir)
	ca, created, err := pki.EnsureCA(paths, logger)
	if err != nil {
		return nil, "", err
	}
	identity, err := os.ReadFile(paths.Identity)
	if err != nil {
		return nil, "", fmt.Errorf("read ca identity: %w", err)
	}
	logger.Info("internal ca ready",
		slog.String("dir", dir),
		slog.Bool("bootstrapped", created),
	)
	return ca, string(identity), nil
}

// buildAuthService wires the auth service from the DB pool, loading the
// Ed25519 signing key from disk or generating an ephemeral one for dev.
func buildAuthService(logger *slog.Logger, pool *sql.DB) (*auth.Service, error) {
	priv, err := loadOrGenerateSigningKey(logger)
	if err != nil {
		return nil, err
	}
	signer := auth.NewSigner(priv, jwtIssuer, accessTokenTTL, refreshTokenTTL, sessionInactivity)
	return auth.NewService(authrepo.New(pool), signer, auth.Argon2idParams{}), nil
}

func loadOrGenerateSigningKey(logger *slog.Logger) (ed25519.PrivateKey, error) {
	path := os.Getenv(jwtKeyPathEnvVar)
	if path == "" {
		logger.Warn("jwt signing key: generating ephemeral key (set HELLING_JWT_PRIVATE_KEY_PATH for persistence)")
		_, priv, err := auth.GenerateKey()
		if err != nil {
			return nil, fmt.Errorf("generate ed25519 key: %w", err)
		}
		return priv, nil
	}
	raw, err := os.ReadFile(path) //nolint:gosec // path is operator-controlled
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("pem decode %s: empty", path)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse pkcs8 %s: %w", path, err)
	}
	priv, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("parse %s: not an Ed25519 private key", path)
	}
	logger.Info("jwt signing key loaded", slog.String("path", path))
	return priv, nil
}

// Keep database/sql imported for symbol stability across test stubs of openDB.
var _ = (*sql.DB)(nil)

func main() {
	logger := newLogger(stderr)

	cfg, err := parseFlags(osArgs[1:], stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			exitFunc(0)
			return
		}
		exitFunc(2)
		return
	}

	exitFunc(run(logger, cfg, defaultServe))
}
