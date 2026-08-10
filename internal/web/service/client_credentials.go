package service

import (
	"strings"

	"github.com/google/uuid"
)

// RegenerateCredentialsByEmail issues a fresh UUID/password/auth secret and a
// fresh SubID for a client ("revoke this share link"), without touching
// traffic, quota, expiry, enable state, or any other field — Update/
// UpdateByEmail only rotate the fields explicitly set on the passed struct,
// so setting exactly these four is what keeps everything else untouched.
// Scoped to the ID/Password/Auth/SubID credential shape shared by VLESS/
// VMess/Trojan/Shadowsocks/Hysteria; WireGuard keys and MTProto secrets are
// out of scope here.
func (s *ClientService) RegenerateCredentialsByEmail(inboundSvc *InboundService, email string) (newID, newSubID string, err error) {
	rec, err := s.GetRecordByEmail(nil, email)
	if err != nil {
		return "", "", err
	}
	updated := *rec.ToClient()
	updated.ID = uuid.NewString()
	updated.Password = strings.ReplaceAll(uuid.NewString(), "-", "")
	updated.Auth = strings.ReplaceAll(uuid.NewString(), "-", "")
	updated.SubID = uuid.NewString()
	if _, err := s.UpdateByEmail(inboundSvc, email, updated); err != nil {
		return "", "", err
	}
	return updated.ID, updated.SubID, nil
}
