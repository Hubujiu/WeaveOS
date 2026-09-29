package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"time"
)

type RequestMetadata struct{ RequestID, ClientIP, UserAgent string }

type metadataKey struct{}

var requestIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Metadata returns a value copy; callers cannot change facts held in context.
func Metadata(ctx context.Context) RequestMetadata {
	m, _ := ctx.Value(metadataKey{}).(RequestMetadata)
	return m
}

// Prepare is shared by the process HTTP entry and a directly mounted Web adapter.
// A request is normalized once, before any handler receives it.
func Prepare(w http.ResponseWriter, r *http.Request, hosts []string) (*http.Request, error) {
	if _, ok := r.Context().Value(metadataKey{}).(RequestMetadata); ok {
		return r, nil
	}
	m := RequestMetadata{UserAgent: r.UserAgent()}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	peer, err := netip.ParseAddr(host)
	trusted := false
	if err == nil && peer.Zone() == "" {
		m.ClientIP = peer.String()
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		for _, candidate := range hosts {
			addresses := []netip.Addr{}
			if ip, err := netip.ParseAddr(candidate); err == nil {
				addresses = append(addresses, ip)
			} else {
				addresses, _ = net.DefaultResolver.LookupNetIP(ctx, "ip", candidate)
			}
			for _, ip := range addresses {
				if ip.Unmap() == peer.Unmap() {
					trusted = true
					break
				}
			}
			if trusted {
				break
			}
		}
	}
	if trusted {
		if values := r.Header.Values("X-Request-Id"); len(values) == 1 && requestIDPattern.MatchString(values[0]) {
			m.RequestID = values[0]
		}
		if values := r.Header.Values("X-Forwarded-For"); len(values) == 1 {
			if ip, err := netip.ParseAddr(values[0]); err == nil && ip.Zone() == "" {
				m.ClientIP = ip.String()
			}
		}
	}
	if m.RequestID == "" {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return r, err
		}
		m.RequestID = hex.EncodeToString(random[:])
	}
	w.Header().Set("X-Request-Id", m.RequestID)
	w.Header().Set("Cache-Control", "no-store")
	// Direct BFF operation retains the existing fallback; Nginx hides this header
	// and owns the single all-site policy on deployed ingress responses.
	w.Header().Set("X-Content-Type-Options", "nosniff")
	return r.WithContext(context.WithValue(r.Context(), metadataKey{}, m)), nil
}
