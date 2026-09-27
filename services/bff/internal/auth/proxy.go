package auth

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"time"
)

// The configured ingress overwrites X-Forwarded-For with one client address.
// Untrusted peers, malformed chains and failed name resolution retain the TCP peer.
func (s *Service) clientIP(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return ""
	}
	values := r.Header.Values("X-Forwarded-For")
	if len(values) != 1 {
		return host
	}
	forwarded, err := netip.ParseAddr(values[0])
	if err != nil || forwarded.Zone() != "" {
		return host
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	for _, trusted := range s.TrustedProxyHosts {
		if addr, err := netip.ParseAddr(trusted); err == nil {
			if addr.Unmap() == peer.Unmap() {
				return forwarded.String()
			}
			continue
		}
		addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", trusted)
		if err != nil {
			continue
		}
		for _, addr := range addresses {
			if addr.Unmap() == peer.Unmap() {
				return forwarded.String()
			}
		}
	}
	return host
}
