package main

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordhttp"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/apprecordservice"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appworkflows"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/auth"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/personnel"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/platform/httpserver"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/workflowexecution"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type config struct {
	DatabaseURL, RedisURL, Origin, Generation, AuditKeyID string
	AuditKey                                              []byte
	TrustedProxyHosts                                     []string
	DefinitionKey                                         []byte
	DefinitionKeyID                                       string
	SchemaLimits                                          appschema.Limits
	WorkflowRuntime                                       workflowRuntimeConfig
}

func readConfig(get func(string) string) (config, error) {
	workflow, err := readWorkflowRuntimeConfig(get)
	if err != nil {
		return config{}, err
	}
	cfg := config{DatabaseURL: get("WEAVEOS_DATABASE_URL"), RedisURL: get("WEAVEOS_REDIS_URL"), Origin: get("WEAVEOS_PUBLIC_ORIGIN"), Generation: get("WEAVEOS_SESSION_GENERATION"), AuditKeyID: get("WEAVEOS_AUDIT_KEY_ID"), WorkflowRuntime: workflow}
	if raw := get("WEAVEOS_TRUSTED_PROXY_HOSTS"); raw != "" {
		for _, value := range strings.Split(raw, ",") {
			host := strings.TrimSpace(value)
			if _, err := netip.ParseAddr(host); err != nil && !regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9.-]*[A-Za-z0-9])?$`).MatchString(host) {
				return config{}, errors.New("invalid trusted proxy host")
			}
			cfg.TrustedProxyHosts = append(cfg.TrustedProxyHosts, host)
		}
	}
	if raw := get("WEAVEOS_AUDIT_HMAC_KEY"); raw != "" {
		key, err := base64.StdEncoding.Strict().DecodeString(raw)
		if err != nil {
			return config{}, errors.New("invalid audit key encoding")
		}
		cfg.AuditKey = key
	}
	definitionKey, keyID, lock, statement := get("WEAVEOS_DEFINITION_HMAC_KEY"), get("WEAVEOS_DEFINITION_KEY_ID"), get("WEAVEOS_SCHEMA_LOCK_TIMEOUT_MS"), get("WEAVEOS_SCHEMA_STATEMENT_TIMEOUT_MS")
	if definitionKey != "" || keyID != "" || lock != "" || statement != "" {
		key, e := base64.StdEncoding.Strict().DecodeString(definitionKey)
		l, le := strconv.ParseInt(lock, 10, 32)
		s, se := strconv.ParseInt(statement, 10, 32)
		if e != nil || len(key) < 32 || !regexp.MustCompile(`^[A-Za-z0-9_-]{1,16}$`).MatchString(keyID) || le != nil || se != nil || l <= 0 || s <= 0 {
			return config{}, errors.New("incomplete or invalid definition configuration")
		}
		cfg.DefinitionKey = key
		cfg.DefinitionKeyID = keyID
		cfg.SchemaLimits = appschema.Limits{LockTimeout: time.Duration(l) * time.Millisecond, StatementTimeout: time.Duration(s) * time.Millisecond}
	}
	return cfg, nil
}
func buildHost(startupCtx, processCtx context.Context, cfg config) (*bffHost, error) {
	if startupCtx == nil || processCtx == nil || startupCtx.Err() != nil || processCtx.Err() != nil {
		return nil, errors.New("BFF host requires active startup and process contexts")
	}
	if cfg.WorkflowRuntime.Enabled && (cfg.SchemaLimits.LockTimeout < time.Millisecond || cfg.SchemaLimits.StatementTimeout < time.Millisecond) {
		return nil, errors.New("incomplete or invalid workflow schema limits")
	}
	host := &bffHost{processCtx: processCtx}
	if !cfg.WorkflowRuntime.Enabled && cfg.DatabaseURL == "" && cfg.RedisURL == "" && cfg.Origin == "" && cfg.Generation == "" && cfg.AuditKeyID == "" && len(cfg.AuditKey) == 0 {
		host.Handler = httpserver.NewHandler(nil)
		return host, nil
	}
	parsed, err := url.Parse(cfg.Origin)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || cfg.DatabaseURL == "" || cfg.RedisURL == "" || !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(cfg.Generation) || !regexp.MustCompile(`^[A-Za-z0-9_-]{1,16}$`).MatchString(cfg.AuditKeyID) || len(cfg.AuditKey) < 32 {
		return nil, errors.New("incomplete or invalid authentication configuration")
	}
	pool, err := pgxpool.New(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return nil, errors.New("invalid authentication database configuration")
	}
	sessions := session.NewStore(cfg.RedisURL, cfg.Generation)
	queryOptions, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		_ = sessions.Close()
		pool.Close()
		return nil, errors.New("invalid query Redis configuration")
	}
	queryRedis := redis.NewClient(queryOptions)
	host.closeResources = func() error {
		failed := false
		if err := queryRedis.Close(); err != nil {
			failed = true
		}
		if err := sessions.Close(); err != nil {
			failed = true
		}
		pool.Close()
		if failed {
			return errBFFHostCleanup
		}
		return nil
	}
	s := &auth.Service{Pool: pool, Sessions: sessions, Origin: cfg.Origin, AuditKeyID: cfg.AuditKeyID, AuditKey: cfg.AuditKey, Logger: slog.Default(), TrustedProxyHosts: cfg.TrustedProxyHosts}
	people := &personnel.Application{Pool: pool, Queries: personnel.NewQueryContextStore(queryRedis, cfg.Generation)}
	s.Personnel = &personnel.Service{Application: people, Authenticator: session.Authenticator{Sessions: sessions, DB: pool, Origin: cfg.Origin}, Logger: slog.Default(), TrustedProxyHosts: cfg.TrustedProxyHosts}
	apps := &applications.Service{Application: &applications.Application{Pool: pool}, Authenticator: session.Authenticator{Sessions: sessions, DB: pool, Origin: cfg.Origin}, Logger: slog.Default(), TrustedProxyHosts: cfg.TrustedProxyHosts}
	apps.Definitions = &appstructure.Service{Application: &appstructure.Application{Pool: pool, ConfirmationKey: cfg.DefinitionKey, ConfirmationKeyID: cfg.DefinitionKeyID, Limits: cfg.SchemaLimits, Dependencies: appstructure.LocalRegistry{}, References: appstructure.CurrentSources{}, CandidateRedis: queryRedis, CandidateNamespace: cfg.Generation, RecordAccess: apprecordhttp.ResolveAccess}, Authenticator: session.Authenticator{Sessions: sessions, DB: pool, Origin: cfg.Origin}, TrustedProxyHosts: cfg.TrustedProxyHosts}
	workflows := &appworkflows.Application{Pool: pool, Limits: cfg.SchemaLimits}
	apps.Workflows = &appworkflows.Service{Application: workflows, Authenticator: session.Authenticator{Sessions: sessions, DB: pool, Origin: cfg.Origin}, TrustedProxyHosts: cfg.TrustedProxyHosts}
	records := apprecordservice.New(pool, queryRedis, cfg.Generation)
	records.Limits = cfg.SchemaLimits
	apps.Records = &apprecordhttp.Service{Records: records, Authenticator: session.Authenticator{Sessions: sessions, DB: pool, Origin: cfg.Origin}, TrustedProxyHosts: cfg.TrustedProxyHosts}
	s.Applications = apps
	s.InvitationBegin = func(ctx context.Context, p session.Principal, version string) (pgx.Tx, error) {
		tx, err := people.BeginQueryWrite(ctx, p, version)
		for _, entry := range []struct {
			err  error
			code string
		}{{personnel.ErrDenied, "COMMON_PERMISSION_DENIED"}, {personnel.ErrConflict, "PERSONNEL_CONFLICT"}, {personnel.ErrQueryChanged, "COMMON_QUERY_CHANGED"}, {personnel.ErrQueryContextExpired, "COMMON_QUERY_CONTEXT_EXPIRED"}} {
			if errors.Is(err, entry.err) {
				return nil, &auth.Failure{Code: entry.code}
			}
		}
		if errors.Is(err, personnel.ErrQueryBusy) {
			return nil, &auth.Failure{Code: "COMMON_SERVICE_UNAVAILABLE", Reason: "QUERY_BUSY"}
		}
		return tx, err
	}
	host.authReady = s.Ready
	if err := s.Ready(startupCtx); err != nil {
		return host.failStartup(errors.New("authentication dependencies unavailable"))
	}
	if startupCtx.Err() != nil || processCtx.Err() != nil {
		return host.failStartup(errBFFHostUnavailable)
	}
	if cfg.WorkflowRuntime.Enabled {
		rpc, err := connectWorkflowRPC(startupCtx, cfg.WorkflowRuntime, grpc.NewClient)
		if err != nil {
			return host.failStartup(err)
		}
		host.rpc = rpc
		workflows.DeploymentClient = rpc.Deployment
		execution := &workflowexecution.Worker{Pool: pool, Client: rpc.Execution, RPCTimeout: cfg.WorkflowRuntime.RPCTimeout, Limits: cfg.SchemaLimits}
		if startupCtx.Err() != nil {
			return host.failStartup(errBFFHostUnavailable)
		}
		workers, err := startWorkflowWorkers(processCtx, workflows.RunPublications, execution.Run)
		if err != nil {
			return host.failStartup(errors.New("workflow workers unavailable"))
		}
		host.workers = workers
		workflows.RuntimeReady = host.runtimeReady
		records.RuntimeReady = host.runtimeReady
	}
	host.Handler = httpserver.NewHandler(host.ready, s)
	return host, nil
}

// The compatibility entry point shares the full host composition, with workers
// owned by the returned cleanup function rather than the startup context.
func buildHandler(ctx context.Context, cfg config) (http.Handler, func(), error) {
	host, err := buildHost(ctx, context.Background(), cfg)
	if err != nil {
		return nil, nil, err
	}
	return host.Handler, func() { _ = host.Close() }, nil
}
