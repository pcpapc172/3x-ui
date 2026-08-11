package service

import "testing"

// TestResolveFallbackHost_PrecedenceAndLoopbackFiltering is the regression
// test for the "bot showed https://localhost:.../sub/..." bug report: a
// real subscription request's host must be usable as a fallback when no Sub
// Domain/Web Domain is configured, but loopback/health-check hosts must
// never poison that cache, and an explicit Web Domain must still win.
func TestResolveFallbackHost_PrecedenceAndLoopbackFiltering(t *testing.T) {
	setupBulkDB(t)
	settingSvc := &SettingService{}

	// No web domain configured, nothing observed yet: falls through to the
	// caller-supplied fallback, then "localhost".
	if host, scheme := ResolveFallbackHost(settingSvc, "caller-fallback.example"); host != "caller-fallback.example" || scheme != "" {
		t.Fatalf("no config/observed: got (%q,%q), want (\"caller-fallback.example\",\"\")", host, scheme)
	}
	if host, _ := ResolveFallbackHost(settingSvc, ""); host != "localhost" {
		t.Fatalf("no config/observed/fallback: got %q, want \"localhost\"", host)
	}

	// Loopback/local hosts must never be cached as the public fallback.
	for _, loopback := range []string{"localhost", "127.0.0.1", "0.0.0.0", "::1"} {
		RecordObservedPublicHost(loopback, "https")
	}
	if host, _ := ObservedPublicHost(); host != "" {
		t.Fatalf("loopback host was cached: got %q, want empty", host)
	}

	// A real subscription request's host is a valid fallback, and its
	// scheme carries through when it's the source actually used.
	RecordObservedPublicHost("real-subscribers.example.com", "https")
	if host, scheme := ResolveFallbackHost(settingSvc, ""); host != "real-subscribers.example.com" || scheme != "https" {
		t.Fatalf("observed host: got (%q,%q), want (\"real-subscribers.example.com\",\"https\")", host, scheme)
	}

	// An explicit Web Domain setting still wins over the observed host.
	if err := settingSvc.setString("webDomain", "configured.example.com"); err != nil {
		t.Fatalf("setString(webDomain): %v", err)
	}
	if host, scheme := ResolveFallbackHost(settingSvc, ""); host != "configured.example.com" || scheme != "" {
		t.Fatalf("web domain configured: got (%q,%q), want (\"configured.example.com\",\"\")", host, scheme)
	}
}
