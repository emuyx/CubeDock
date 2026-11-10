package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/emuyx/cubedock/internal/config"
	"github.com/emuyx/cubedock/internal/session"
)

func diagnosticsByID(checks []DiagnosticCheck) map[string]DiagnosticCheck {
	out := make(map[string]DiagnosticCheck, len(checks))
	for _, check := range checks {
		out[check.ID] = check
	}
	return out
}

func TestDiagnoseHealthyStoppedManager(t *testing.T) {
	manager := newTestManager(t)
	cfg := config.Defaults()
	cfg.LimboPacksDir = t.TempDir()
	if err := manager.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	addTestAccounts(t, manager, 2)
	manager.pingFn = func(context.Context, config.Config) (string, error) {
		return "1.21.130.0", nil
	}
	lockDir := t.TempDir()
	manager.claimSession = func(token string) (*session.Lease, error) {
		return session.ClaimIn(lockDir, token)
	}

	checks := diagnosticsByID(manager.Diagnose(context.Background()))
	want := map[string]string{
		"config": DiagnosticOK, "data_dir": DiagnosticOK, "upstream": DiagnosticOK,
		"limbo": DiagnosticSkip, "packs": DiagnosticOK, "accounts": DiagnosticOK,
		"mode": DiagnosticOK, "locks": DiagnosticOK,
	}
	for id, status := range want {
		if checks[id].Status != status {
			t.Fatalf("%s status = %q, want %q (%+v)", id, checks[id].Status, status, checks[id])
		}
	}
	for _, check := range checks {
		if check.Status == "" || check.Label == "" {
			t.Fatalf("incomplete check: %+v", check)
		}
	}
}

func TestDiagnoseReportsFailuresWithoutSideEffects(t *testing.T) {
	manager := newTestManager(t)
	cfg := config.Defaults()
	cfg.LimboPacksDir = filepath.Join(t.TempDir(), "missing-packs")
	if err := manager.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	manager.pingFn = func(context.Context, config.Config) (string, error) {
		return "", errors.New("connection refused")
	}

	checks := diagnosticsByID(manager.Diagnose(context.Background()))
	if checks["upstream"].Status != DiagnosticFail {
		t.Fatalf("upstream = %+v", checks["upstream"])
	}
	if checks["packs"].Status != DiagnosticWarn {
		t.Fatalf("packs = %+v", checks["packs"])
	}
	if checks["accounts"].Status != DiagnosticWarn {
		t.Fatalf("accounts = %+v", checks["accounts"])
	}
	if checks["locks"].Status != DiagnosticSkip {
		t.Fatalf("locks = %+v", checks["locks"])
	}
	if _, err := os.Stat(cfg.LimboPacksDir); !os.IsNotExist(err) {
		t.Fatal("diagnostics created the resource pack directory")
	}
}

func TestDiagnoseSkipsLocksWhileRunning(t *testing.T) {
	manager := newTestManager(t)
	addTestAccounts(t, manager, 1)
	manager.pingFn = func(context.Context, config.Config) (string, error) {
		return "1.21.130.0", nil
	}
	manager.mu.Lock()
	manager.cancel = func() {}
	manager.mu.Unlock()
	defer func() {
		manager.mu.Lock()
		manager.cancel = nil
		manager.mu.Unlock()
	}()
	manager.claimSession = func(string) (*session.Lease, error) {
		t.Fatal("diagnostics claimed a session lock while running")
		return nil, nil
	}

	checks := diagnosticsByID(manager.Diagnose(context.Background()))
	if checks["locks"].Status != DiagnosticSkip {
		t.Fatalf("locks = %+v", checks["locks"])
	}
}

func TestDiagnoseReportsBusyAndBrokenLocks(t *testing.T) {
	manager := newTestManager(t)
	addTestAccounts(t, manager, 2)
	manager.pingFn = func(context.Context, config.Config) (string, error) {
		return "1.21.130.0", nil
	}
	manager.claimSession = func(token string) (*session.Lease, error) {
		if token == "token-00" {
			return nil, session.ErrTokenBusy
		}
		return nil, errors.New("lock directory unavailable")
	}

	checks := diagnosticsByID(manager.Diagnose(context.Background()))
	if checks["locks"].Status != DiagnosticFail {
		t.Fatalf("locks = %+v", checks["locks"])
	}
}
