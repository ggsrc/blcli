package bootstrap

import (
	"errors"
	"strings"
	"testing"
)

func TestMatchFailureHintTerraformBackend(t *testing.T) {
	hint := matchFailureHint("apply terraform", "failed to initialize terraform backend")
	if hint == nil || !strings.Contains(hint.Title, "backend") {
		t.Fatalf("expected backend hint, got %#v", hint)
	}
}

func TestMatchFailureHintValidation(t *testing.T) {
	hint := matchFailureHint("init", "validation failed: billingaccountid is required")
	if hint == nil || !strings.Contains(hint.Title, "validation") {
		t.Fatalf("expected validation hint, got %#v", hint)
	}
}

func TestWrapApplyErrorNil(t *testing.T) {
	if err := WrapApplyError("apply", nil); err != nil {
		t.Fatal("expected nil")
	}
}

func TestWrapApplyErrorPreservesError(t *testing.T) {
	orig := errors.New("terraform apply failed: backend not initialized")
	if err := WrapApplyError("apply terraform", orig); !errors.Is(err, orig) {
		t.Fatalf("expected original error, got %v", err)
	}
}
