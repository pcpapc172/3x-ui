package service

import "sync"

var (
	observedHostMu     sync.RWMutex
	observedHost       string
	observedHostScheme string
)

// RecordObservedPublicHost caches the host+scheme a real subscription
// request just arrived on, for BuildSubscriptionURLs's fallback when no Sub
// Domain is explicitly configured in settings. Loopback hosts are ignored —
// they're never useful as a public fallback and would only ever come from a
// health check or local debugging request, not a real subscriber.
func RecordObservedPublicHost(host, scheme string) {
	if host == "" || host == "localhost" || host == "0.0.0.0" || host == "::1" ||
		len(host) >= 4 && host[:4] == "127." {
		return
	}
	observedHostMu.Lock()
	observedHost = host
	observedHostScheme = scheme
	observedHostMu.Unlock()
}

// ObservedPublicHost returns the most recently recorded public host+scheme,
// or ("", "") if none has been observed yet.
func ObservedPublicHost() (host, scheme string) {
	observedHostMu.RLock()
	defer observedHostMu.RUnlock()
	return observedHost, observedHostScheme
}
