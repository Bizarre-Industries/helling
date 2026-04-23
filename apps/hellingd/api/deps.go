package api

import (
	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/auth"
)

// Deps carries optional runtime dependencies for Helling API handlers.
// A zero Deps keeps the stubbed handlers, which is what the unit tests for
// the Huma spike rely on. Production wiring passes the real services.
type Deps struct {
	// Auth is the auth service powering setup/login/logout/refresh.
	// When nil, stub handlers are registered instead.
	Auth *auth.Service
}

// HasAuth reports whether a real auth service is wired in.
func (d Deps) HasAuth() bool { return d.Auth != nil }
