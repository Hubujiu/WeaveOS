package session_test

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
)

func redisCommand(t *testing.T, rawURL string, args ...string) any {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "redis" || u.Hostname() != "127.0.0.1" {
		t.Fatalf("test Redis must be local: %q", u.Host)
	}
	conn, err := net.DialTimeout("tcp", u.Host, 5*time.Second)
	if err != nil {
		t.Fatalf("connect isolated Redis: %v", err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	send := func(parts ...string) any {
		if _, err := fmt.Fprintf(writer, "*%d\r\n", len(parts)); err != nil {
			t.Fatal(err)
		}
		for _, part := range parts {
			if _, err := fmt.Fprintf(writer, "$%d\r\n%s\r\n", len(part), part); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Flush(); err != nil {
			t.Fatal(err)
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		line = strings.TrimSuffix(line, "\r\n")
		switch line[0] {
		case '+':
			return line[1:]
		case ':':
			value, err := strconv.ParseInt(line[1:], 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			return value
		case '$':
			size, err := strconv.Atoi(line[1:])
			if err != nil {
				t.Fatal(err)
			}
			if size == -1 {
				return nil
			}
			body := make([]byte, size+2)
			if _, err := io.ReadFull(reader, body); err != nil {
				t.Fatal(err)
			}
			return string(body[:size])
		case '-':
			t.Fatalf("Redis returned error: %s", line[1:])
		default:
			t.Fatalf("unexpected Redis response: %q", line)
		}
		return nil
	}
	database := strings.TrimPrefix(u.Path, "/")
	if database != "" && database != "0" {
		send("SELECT", database)
	}
	return send(args...)
}

func isolatedRedis(t *testing.T) (string, string) {
	t.Helper()
	rawURL := os.Getenv("WEAVEOS_TEST_REDIS_URL")
	if rawURL == "" {
		t.Fatal("WEAVEOS_TEST_REDIS_URL must point to isolated Redis 8.2")
	}
	info, ok := redisCommand(t, rawURL, "INFO", "server").(string)
	if !ok || !strings.Contains(info, "redis_version:8.2.") {
		t.Fatal("real Redis 8.2 is required for session acceptance")
	}
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		t.Fatal(err)
	}
	return rawURL, "test-" + hex.EncodeToString(random)
}

func approvedRecord() session.Record {
	return session.Record{
		UserID:      "550e8400-e29b-41d4-a716-446655440001",
		SessionRef:  "550e8400-e29b-41d4-a716-446655440002",
		AuthVersion: "1",
	}
}

func keyFor(generation, sid string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(sid)
	if err != nil {
		return "", err
	}
	if len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != sid {
		return "", fmt.Errorf("SID is not canonical 32-byte Base64URL")
	}
	digest := sha256.Sum256(raw)
	return "ems:auth:session:" + generation + ":v1:" + hex.EncodeToString(digest[:]), nil
}

func TestCreateLoadUsesApprovedRedisShapeAndOneHourTTL(t *testing.T) {
	rawURL, generation := isolatedRedis(t)
	store := session.NewStore(rawURL, generation)
	ctx := context.Background()
	sid, csrf, err := store.Create(ctx, approvedRecord())
	if err != nil {
		t.Fatalf("create approved server-side session: %v", err)
	}
	key, err := keyFor(generation, sid)
	if err != nil {
		t.Fatal(err)
	}
	csrfRaw, err := base64.RawURLEncoding.DecodeString(csrf)
	if err != nil || len(csrfRaw) != 32 || csrf == sid {
		t.Fatal("CSRF and SID must be distinct canonical random values")
	}
	t.Cleanup(func() { redisCommand(t, rawURL, "DEL", key) })
	ttl, ok := redisCommand(t, rawURL, "PTTL", key).(int64)
	if !ok || ttl < 3590000 || ttl > 3600000 {
		t.Errorf("new Session TTL = %v ms, want approximately 3600000", ttl)
	}
	value, ok := redisCommand(t, rawURL, "GET", key).(string)
	if !ok {
		t.Fatal("Session missing from Redis")
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(value), &fields); err != nil {
		t.Fatal(err)
	}
	if fields["schema_version"] != float64(1) || fields["user_id"] != approvedRecord().UserID || fields["auth_version"] != "1" {
		t.Errorf("required session facts differ from approved schema: %v", fields)
	}
	if strings.Contains(value, sid) || strings.Contains(value, csrf) {
		t.Error("Redis JSON contains a replayable SID or CSRF token")
	}
	loaded, err := store.Load(ctx, sid)
	if err != nil || loaded.UserID != approvedRecord().UserID || loaded.SessionRef != approvedRecord().SessionRef || loaded.AuthVersion != "1" {
		t.Errorf("load approved session: %+v, %v", loaded, err)
	}
}

func TestTouchSlidesTTLButNeverRevivesRevokedSession(t *testing.T) {
	rawURL, generation := isolatedRedis(t)
	store := session.NewStore(rawURL, generation)
	ctx := context.Background()
	sid, _, err := store.Create(ctx, approvedRecord())
	if err != nil {
		t.Fatalf("create session before touch/revoke: %v", err)
	}
	key, err := keyFor(generation, sid)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { redisCommand(t, rawURL, "DEL", key) })
	redisCommand(t, rawURL, "PEXPIRE", key, "5000")
	loaded, err := store.Load(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	touched, err := store.Touch(ctx, sid, loaded)
	if err != nil || !touched {
		t.Fatalf("successful activity must touch session: touched=%t error=%v", touched, err)
	}
	ttl, ok := redisCommand(t, rawURL, "PTTL", key).(int64)
	if !ok || ttl < 3590000 || ttl > 3600000 {
		t.Errorf("sliding TTL = %v ms, want approximately 3600000", ttl)
	}
	revoked, err := store.Revoke(ctx, sid)
	if err != nil || !revoked {
		t.Fatalf("logout must delete the session: revoked=%t error=%v", revoked, err)
	}
	touched, err = store.Touch(ctx, sid, loaded)
	if err != nil || touched || redisCommand(t, rawURL, "EXISTS", key) != int64(0) {
		t.Errorf("old session revived after logout: touched=%t error=%v", touched, err)
	}
}

func TestConcurrentTouchAndRevokeCannotResurrectSession(t *testing.T) {
	rawURL, generation := isolatedRedis(t)
	store := session.NewStore(rawURL, generation)
	ctx := context.Background()
	sid, _, err := store.Create(ctx, approvedRecord())
	if err != nil {
		t.Fatalf("create session before race: %v", err)
	}
	key, err := keyFor(generation, sid)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { redisCommand(t, rawURL, "DEL", key) })
	loaded, err := store.Load(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); <-start; _, err := store.Touch(ctx, sid, loaded); errors <- err }()
	go func() { defer wg.Done(); <-start; _, err := store.Revoke(ctx, sid); errors <- err }()
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("concurrent store operation failed: %v", err)
		}
	}
	if redisCommand(t, rawURL, "EXISTS", key) != int64(0) {
		t.Error("revoked SID was resurrected by concurrent touch")
	}
}

func TestLoadRejectsTrailingJSONDocument(t *testing.T) {
	rawURL, generation := isolatedRedis(t)
	store := session.NewStore(rawURL, generation)
	ctx := context.Background()
	sid, _, err := store.Create(ctx, approvedRecord())
	if err != nil { t.Fatal(err) }
	key, err := keyFor(generation, sid)
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { redisCommand(t, rawURL, "DEL", key) })
	raw := redisCommand(t, rawURL, "GET", key).(string)
	redisCommand(t, rawURL, "SET", key, raw+` {"schema_version":99}`, "PX", "5000")
	if _, err := store.Load(ctx, sid); err == nil {
		t.Fatal("multiple JSON documents must not be accepted as a Session record")
	}
}

func TestTouchRefusesCorruptedTimestampsWithoutRenewingTTL(t *testing.T) {
	rawURL, generation := isolatedRedis(t)
	store := session.NewStore(rawURL, generation)
	ctx := context.Background()
	sid, _, err := store.Create(ctx, approvedRecord())
	if err != nil { t.Fatal(err) }
	key, err := keyFor(generation, sid)
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { redisCommand(t, rawURL, "DEL", key) })
	loaded, err := store.Load(ctx, sid)
	if err != nil { t.Fatal(err) }
	var malformed map[string]any
	if err := json.Unmarshal([]byte(redisCommand(t, rawURL, "GET", key).(string)), &malformed); err != nil { t.Fatal(err) }
	malformed["created_at_unix_ms"] = -1
	encoded, err := json.Marshal(malformed)
	if err != nil { t.Fatal(err) }
	redisCommand(t, rawURL, "SET", key, string(encoded), "PX", "5000")
	touched, err := store.Touch(ctx, sid, loaded)
	if err != nil { t.Fatal(err) }
	if touched { t.Fatal("corrupted timestamp record must not be extended by an earlier valid load") }
	ttl := redisCommand(t, rawURL, "PTTL", key).(int64)
	if ttl > 5000 { t.Fatalf("invalid record TTL was renewed: %d ms", ttl) }
}

func TestLoadAndTouchRejectOverlongTTL(t *testing.T) {
	rawURL, generation := isolatedRedis(t)
	store := session.NewStore(rawURL, generation)
	ctx := context.Background()
	sid, _, err := store.Create(ctx, approvedRecord())
	if err != nil { t.Fatal(err) }
	key, err := keyFor(generation, sid)
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { redisCommand(t, rawURL, "DEL", key) })
	loaded, err := store.Load(ctx, sid)
	if err != nil { t.Fatal(err) }
	redisCommand(t, rawURL, "PEXPIRE", key, "7200000")
	if _, err := store.Load(ctx, sid); err == nil {
		t.Error("TTL exceeding the approved one-hour maximum must be rejected")
	}
	touched, err := store.Touch(ctx, sid, loaded)
	if err != nil { t.Fatal(err) }
	if touched { t.Error("overlong TTL must not be silently accepted by touch") }
}
