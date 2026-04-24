package pki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Bizarre-Industries/Helling/apps/hellingd/internal/repo/authrepo"
)

// Renewer drives the per-user certificate renewal loop per
// docs/spec/internal-ca.md §4.1: certs are rotated when their remaining
// validity drops to UserCertRenewalThreshold (60 days). Stateless across
// ticks; queries authrepo each pass.
type Renewer struct {
	Issuer *Issuer
	Repo   *authrepo.Repo
	Logger *slog.Logger
	Now    func() time.Time // optional; defaults to time.Now
	Limit  int              // max certs renewed per tick (default 100)
}

// Tick runs one pass: lists active certs whose ExpiresAt is within the
// renewal threshold, supersedes them, and re-issues. Returns the number of
// rows successfully renewed plus the first error encountered (if any).
func (r *Renewer) Tick(ctx context.Context) (int, error) {
	if r == nil || r.Issuer == nil || r.Repo == nil {
		return 0, errors.New("pki.Renewer: not configured")
	}
	now := time.Now
	if r.Now != nil {
		now = r.Now
	}
	limit := r.Limit
	if limit <= 0 {
		limit = 100
	}
	cutoff := now().Add(UserCertRenewalThreshold)
	rows, err := r.Repo.ListExpiringUserCertificates(ctx, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("pki.Renewer: list: %w", err)
	}
	renewed := 0
	for i := range rows {
		row := rows[i]
		user, err := r.Repo.GetUserByID(ctx, row.UserID)
		if err != nil {
			r.logRenewalFailure(&row, err)
			continue
		}
		if err := r.Repo.SupersedeUserCertificate(ctx, row.ID); err != nil {
			r.logRenewalFailure(&row, err)
			continue
		}
		if err := r.Issuer.IssueForUser(ctx, user.ID, user.Username); err != nil {
			r.logRenewalFailure(&row, err)
			continue
		}
		r.recordAudit(ctx, user.ID, row.SerialNumber)
		renewed++
	}
	return renewed, nil
}

// Run loops Tick on the given interval until ctx is canceled. Errors are
// logged but never crash the loop. Runs one immediate tick at startup.
func (r *Renewer) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	if _, err := r.Tick(ctx); err != nil && r.Logger != nil {
		r.Logger.ErrorContext(ctx, "pki renewer tick", slog.Any("err", err))
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if _, err := r.Tick(ctx); err != nil && r.Logger != nil {
				r.Logger.ErrorContext(ctx, "pki renewer tick", slog.Any("err", err))
			}
		}
	}
}

func (r *Renewer) logRenewalFailure(row *authrepo.UserCertificate, err error) {
	if r.Logger == nil {
		return
	}
	r.Logger.Warn("user cert renewal failed",
		slog.String("user_id", row.UserID),
		slog.String("serial", row.SerialNumber),
		slog.Any("err", err),
	)
}

func (r *Renewer) recordAudit(ctx context.Context, userID, oldSerial string) {
	meta, _ := json.Marshal(map[string]any{
		"old_serial": oldSerial,
		"reason":     "renewal_threshold",
	})
	_ = r.Repo.RecordEvent(ctx, userID, "pki.cert_renewed", "", "", string(meta))
}
