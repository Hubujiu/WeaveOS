package main

import "testing"

func TestConfiguredProxyHostsAreReadAndInvalidHostsRejected(t *testing.T) {
	cfg, err := readConfig(func(k string) string {
		if k == "WEAVEOS_TRUSTED_PROXY_HOSTS" {
			return "nginx,localhost"
		}
		return ""
	})
	if err != nil || len(cfg.TrustedProxyHosts) != 2 || cfg.TrustedProxyHosts[0] != "nginx" {
		t.Fatal("explicit proxy service names must be loaded")
	}
	if _, err = readConfig(func(k string) string {
		if k == "WEAVEOS_TRUSTED_PROXY_HOSTS" {
			return "https://untrusted/"
		}
		return ""
	}); err == nil {
		t.Fatal("proxy settings must contain host names or IP addresses, not URLs")
	}
}
