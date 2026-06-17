package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sunpe/doraemon/internal/store"
)

func TestTokenLifecycleStoresOnlyHashAndAuthenticatesUser(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "doraemon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.CreateUser("alice", []string{"readonly"}); err != nil {
		t.Fatal(err)
	}
	created, err := db.CreateToken("alice", "laptop", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if created.Plaintext == "" {
		t.Fatal("expected plaintext token to be returned once")
	}

	raw, err := db.DebugRawToken(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if raw.Hash == "" {
		t.Fatal("expected stored token hash")
	}
	if raw.Hash == created.Plaintext {
		t.Fatal("stored token hash must not equal plaintext")
	}
	if raw.HashContains(created.Plaintext) {
		t.Fatal("stored token record must not contain plaintext token")
	}

	principal, err := db.AuthenticateToken(created.Plaintext, time.Now())
	if err != nil {
		t.Fatalf("AuthenticateToken returned error: %v", err)
	}
	if principal.User != "alice" || principal.TokenID != created.ID {
		t.Fatalf("unexpected principal: %+v", principal)
	}
}

func TestAuthenticateRejectsExpiredRevokedAndDisabledUser(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "doraemon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.CreateUser("alice", []string{"readonly"}); err != nil {
		t.Fatal(err)
	}
	expired, err := db.CreateToken("alice", "old", time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AuthenticateToken(expired.Plaintext, time.Now()); err == nil {
		t.Fatal("expected expired token to be rejected")
	}

	active, err := db.CreateToken("alice", "active", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.RevokeToken(active.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AuthenticateToken(active.Plaintext, time.Now()); err == nil {
		t.Fatal("expected revoked token to be rejected")
	}

	disabled, err := db.CreateToken("alice", "disabled-user", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DisableUser("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.AuthenticateToken(disabled.Plaintext, time.Now()); err == nil {
		t.Fatal("expected disabled user to be rejected")
	}
}
