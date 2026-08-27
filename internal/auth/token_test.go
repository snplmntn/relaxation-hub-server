package auth

import (
	"testing"
	"time"

	"github.com/snplmntn/relaxation-hub-server/internal/model"
)

func TestTokenLifetime(t *testing.T) {
	if got := tokenLifetime(model.RoleAdmin); got != 30*24*time.Hour {
		t.Fatalf("expected admin token lifetime of 30 days, got %s", got)
	}
	if got := tokenLifetime(model.RoleSuperAdmin); got != 30*24*time.Hour {
		t.Fatalf("expected super-admin token lifetime of 30 days, got %s", got)
	}
	if got := tokenLifetime(model.RoleClient); got != 24*time.Hour {
		t.Fatalf("expected non-admin token lifetime to remain 24 hours, got %s", got)
	}
}
