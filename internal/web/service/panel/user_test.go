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
