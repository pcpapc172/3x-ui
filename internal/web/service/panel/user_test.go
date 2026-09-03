package panel

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/crypto"
)

// TestCheckUser_RejectsDisabledReseller is the regression test for the
// reported bug: disabling a reseller invalidated their existing session
// (LoginEpoch mismatch) but nothing stopped them from simply logging back
// in with their still-valid password, which let a "disabled" reseller keep
// creating and enabling clients.
func TestCheckUser_RejectsDisabledReseller(t *testing.T) {
	setupBotTestDB(t)
	db := database.GetDB()

	hashed, err := crypto.HashPasswordAsBcrypt("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	reseller := &model.User{
		Username:   "reseller-login",
		Password:   hashed,
		Role:       "reseller",
		LoginEpoch: -1,
	}
	if err := db.Create(reseller).Error; err != nil {
		t.Fatalf("create reseller: %v", err)
	}

	svc := &UserService{}
	if _, err := svc.CheckUser("reseller-login", "secret123", ""); err == nil {
		t.Fatalf("CheckUser succeeded for a disabled reseller, want error")
	}

	if err := db.Model(reseller).Update("login_epoch", 0).Error; err != nil {
		t.Fatalf("re-enable reseller: %v", err)
	}
	if _, err := svc.CheckUser("reseller-login", "secret123", ""); err != nil {
		t.Fatalf("CheckUser failed for an enabled reseller with correct credentials: %v", err)
	}
}

// TestCheckUser_BackfillsResellerPasswordOnFirstLogin: a reseller created before
// the panel stored the owner-visible plaintext gets it captured from the first
// successful local-password login.
func TestCheckUser_BackfillsResellerPasswordOnFirstLogin(t *testing.T) {
	setupBotTestDB(t)
	db := database.GetDB()
	svc := &UserService{}

	hashed, err := crypto.HashPasswordAsBcrypt("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	reseller := &model.User{Username: "bf-reseller", Password: hashed, Role: "reseller"}
	if err := db.Create(reseller).Error; err != nil {
		t.Fatalf("create reseller: %v", err)
	}

	if _, err := svc.CheckUser("bf-reseller", "secret123", ""); err != nil {
		t.Fatalf("CheckUser: %v", err)
	}

	var reloaded model.User
	if err := db.First(&reloaded, reseller.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.ResellerPassword != "secret123" {
		t.Fatalf("reseller_password after first login = %q, want %q", reloaded.ResellerPassword, "secret123")
	}
}

// TestCheckUser_DoesNotOverwriteStoredResellerPassword: once the plaintext is
// stored, a later login must not touch it.
func TestCheckUser_DoesNotOverwriteStoredResellerPassword(t *testing.T) {
	setupBotTestDB(t)
	db := database.GetDB()
	svc := &UserService{}

	hashed, err := crypto.HashPasswordAsBcrypt("secret123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	reseller := &model.User{Username: "keep-reseller", Password: hashed, Role: "reseller", ResellerPassword: "already-stored"}
	if err := db.Create(reseller).Error; err != nil {
		t.Fatalf("create reseller: %v", err)
	}

	if _, err := svc.CheckUser("keep-reseller", "secret123", ""); err != nil {
		t.Fatalf("CheckUser: %v", err)
	}

	var reloaded model.User
	if err := db.First(&reloaded, reseller.Id).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.ResellerPassword != "already-stored" {
		t.Fatalf("reseller_password = %q, want it left as %q", reloaded.ResellerPassword, "already-stored")
	}
}
