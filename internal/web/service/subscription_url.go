package service

import (
	"errors"
	"fmt"
	"strings"
)

// BuildSubscriptionURLs returns a client's subscription page URL (and JSON
// URL, when enabled), using the panel's configured sub-server settings.
// fallbackHost is used only when no sub domain or web domain is configured
// (pass "" for none). Shared by the admin Telegram bot
// (internal/web/service/tgbot) and the reseller shop bot
// (internal/resellerbot) so the host/path/TLS resolution logic lives in
// exactly one place.
func BuildSubscriptionURLs(inboundSvc *InboundService, settingSvc *SettingService, email, fallbackHost string) (subURL, subJsonURL string, err error) {
	_, client, err := inboundSvc.GetClientByEmail(email)
	if err != nil || client == nil {
		return "", "", errors.New("client not found")
	}

	subURI, _ := settingSvc.GetSubURI()
	subJsonURI, _ := settingSvc.GetSubJsonURI()
	subDomain, _ := settingSvc.GetSubDomain()
	subPort, _ := settingSvc.GetSubPort()
	subPath, _ := settingSvc.GetSubPath()
	subJsonPath, _ := settingSvc.GetSubJsonPath()
	subJsonEnable, _ := settingSvc.GetSubJsonEnable()
	subKeyFile, _ := settingSvc.GetSubKeyFile()
	subCertFile, _ := settingSvc.GetSubCertFile()

	tls := subKeyFile != "" && subCertFile != ""
	scheme := "http"
	if tls {
		scheme = "https"
	}

	if subDomain == "" {
		if d, dErr := settingSvc.GetWebDomain(); dErr == nil && d != "" {
			subDomain = d
		} else if fallbackHost != "" {
			subDomain = fallbackHost
		} else {
			subDomain = "localhost"
		}
	}

	host := subDomain
	if !((subPort == 443 && tls) || (subPort == 80 && !tls)) {
		host = fmt.Sprintf("%s:%d", subDomain, subPort)
	}

	if !strings.HasPrefix(subPath, "/") {
		subPath = "/" + subPath
	}
	if !strings.HasSuffix(subPath, "/") {
		subPath += "/"
	}
	if !strings.HasPrefix(subJsonPath, "/") {
		subJsonPath = "/" + subJsonPath
	}
	if !strings.HasSuffix(subJsonPath, "/") {
		subJsonPath += "/"
	}

	if subURI != "" {
		if !strings.HasSuffix(subURI, "/") {
			subURI += "/"
		}
		subURL = subURI + client.SubID
	} else {
		subURL = fmt.Sprintf("%s://%s%s%s", scheme, host, subPath, client.SubID)
	}

	if subJsonURI != "" {
		if !strings.HasSuffix(subJsonURI, "/") {
			subJsonURI += "/"
		}
		subJsonURL = subJsonURI + client.SubID
	} else {
		subJsonURL = fmt.Sprintf("%s://%s%s%s", scheme, host, subJsonPath, client.SubID)
	}
	if !subJsonEnable {
		subJsonURL = ""
	}

	return subURL, subJsonURL, nil
}
