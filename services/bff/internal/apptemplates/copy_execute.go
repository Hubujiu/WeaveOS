package apptemplates

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/applications"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appschema"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/appstructure"
	"github.com/Hubujiu/WeaveOS/services/bff/internal/session"
	"github.com/jackc/pgx/v5"
	"sort"
	"time"
)

func normalizeCopyExecution(in CopyPreflightInput) (CopyPreflightInput, [32]byte, error) {
	bad := func() (CopyPreflightInput, [32]byte, error) { return CopyPreflightInput{}, [32]byte{}, ErrInvalid }
	plan, e := normalizeCopyPlan(CopyPlanInput{Source: in.Source, Target: in.Target})
	if e != nil {
		return bad()
	}
	in.Target = plan.Target
	if len(in.ExpectedSourceFingerprint) != 64 || in.ExpectedTargetStructureVersion < 0 || in.ExpectedTargetPolicyRevision < 0 || in.ExpectedTargetStructureVersion > 9007199254740991 || in.ExpectedTargetPolicyRevision > 9007199254740991 || in.Bindings == nil || in.GroupMappings == nil {
		return bad()
	}
	for _, c := range in.ExpectedSourceFingerprint {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return bad()
		}
	}
	in.Bindings = append([]Binding{}, in.Bindings...)
	in.GroupMappings = append([]CopyGroupMapping{}, in.GroupMappings...)
	seen := map[string]bool{}
	for _, b := range in.Bindings {
		key := b.Kind + "/" + b.SourceID
		if b.Kind != "user" && b.Kind != "department" || !validID(b.SourceID) || !validID(b.TargetID) || seen[key] {
			return bad()
		}
		seen[key] = true
	}
	seen = map[string]bool{}
	for i, g := range in.GroupMappings {
		n, ok := name(g.Name)
		if !ok || !validID(g.SourceGroupID) || seen[g.SourceGroupID] {
			return bad()
		}
		seen[g.SourceGroupID] = true
		in.GroupMappings[i].Name = n
	}
	sort.Slice(in.Bindings, func(i, j int) bool {
		a, b := in.Bindings[i], in.Bindings[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.SourceID < b.SourceID
	})
	sort.Slice(in.GroupMappings, func(i, j int) bool { return in.GroupMappings[i].SourceGroupID < in.GroupMappings[j].SourceGroupID })
	raw, e := json.Marshal(struct {
		Version int
		Kind    string
		Request CopyPreflightInput
	}{1, applications.StructureCopyKind, in})
	if e != nil {
		return bad()
	}
	return in, sha256.Sum256(raw), nil
}
func (s *Service) ExecuteCopy(ctx context.Context, p session.Principal, operationID string, in CopyPreflightInput, metadata applications.Metadata) (applications.Result, error) {
	empty := applications.Result{}
	if s == nil || s.Pool == nil {
		return empty, session.ErrUnavailable
	}
	if !validID(operationID) {
		return empty, ErrInvalid
	}
	in, fingerprint, e := normalizeCopyExecution(in)
	if e != nil {
		return empty, e
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	target, newApp := "", ""
	if in.Target.AppID != nil {
		target = *in.Target.AppID
	} else {
		newApp, e = randomID()
		if e != nil {
			return empty, e
		}
	}
	guard := func(c context.Context, tx pgx.Tx) error {
		source := appstructure.CurrentSources{}
		for _, kind := range []string{"user", "department"} {
			ids := []string{}
			seen := map[string]bool{}
			for _, b := range in.Bindings {
				if b.Kind == kind && !seen[b.TargetID] {
					seen[b.TargetID] = true
					ids = append(ids, b.TargetID)
				}
			}
			sort.Strings(ids)
			if len(ids) > 0 {
				sourceKind := kind
				if kind == "user" {
					sourceKind = "member"
				}
				if err := source.Validate(c, tx, sourceKind, ids); err != nil {
					return err
				}
			}
		}
		return nil
	}
	w, replay, e := (&applications.Application{Pool: s.Pool}).BeginStructureCopyWrite(ctx, p, applications.StructureCopyOptions{OperationID: operationID, Fingerprint: fingerprint, SourceAppID: in.Source.AppID, TargetAppID: target, NewAppID: newApp, Name: in.Target.Name, TargetDirectoryID: in.Target.DirectoryID, SourceGuard: guard})
	if e != nil {
		return empty, e
	}
	defer w.Rollback(context.Background())
	if replay != nil {
		if e = w.Commit(ctx); e != nil {
			return empty, e
		}
		return *replay, nil
	}
	tx := w.Tx()
	full, e := exportManifestInTx(ctx, tx, w.Source)
	if e != nil {
		return empty, e
	}
	selected, e := SelectCopyManifest(full, in.Source.Kind, in.Source.ID)
	if e != nil {
		return empty, applications.ErrMissing
	}
	actual, e := sourceFingerprint(selected)
	if e != nil {
		return empty, e
	}
	if actual != in.ExpectedSourceFingerprint {
		return empty, ErrCopySourceChanged
	}
	policy := w.Target.PolicyRevision
	if target == "" {
		policy = 0
	}
	if in.ExpectedTargetStructureVersion != w.StructureVersion {
		return empty, &appstructure.Error{Code: "APPLICATION_STRUCTURE_CONFLICT", Data: map[string]int64{"currentStructureVersion": w.StructureVersion}}
	}
	if in.ExpectedTargetPolicyRevision != policy {
		return empty, applications.ErrPolicyConflict
	}
	mapped, e := MapExternalReferences(selected, in.Bindings)
	if e != nil {
		return empty, e
	}
	mapped, e = MapCopyGroups(mapped, in.GroupMappings)
	if e != nil {
		return empty, e
	}
	m, e := RemapInternalIDs(mapped, randomID)
	if e != nil {
		return empty, e
	}
	// New-app identity was allocated by the transaction owner; remap app grants too.
	generatedApp := m.Application.ID
	m.Application.ID = w.Target.ID
	for gi := range m.PermissionGroups {
		for j := range m.PermissionGroups[gi].Grants {
			g := &m.PermissionGroups[gi].Grants[j]
			if g.ResourceKind == "application" && g.ResourceID == generatedApp {
				g.ResourceID = w.Target.ID
			}
		}
	}
	root := w.Target.ID
	switch in.Source.Kind {
	case "application":
		m.Application.Name = in.Target.Name
	case "directory":
		for i, d := range selected.Directories {
			if d.ID == in.Source.ID {
				root = m.Directories[i].ID
				m.Directories[i].Name = in.Target.Name
			}
		}
	case "table":
		root = m.Tables[0].ID
		m.Tables[0].Name = in.Target.Name
		m.Tables[0].DirectoryID = in.Target.DirectoryID
		for i := range m.Forms {
			m.Forms[i].DirectoryID = in.Target.DirectoryID
		}
	case "form":
		root = m.Forms[0].ID
		m.Forms[0].Name = in.Target.Name
		m.Forms[0].DirectoryID = in.Target.DirectoryID
		m.Tables[0].DirectoryID = in.Target.DirectoryID
	}
	version := w.StructureVersion + 1
	policy = w.Target.PolicyRevision
	if target != "" && len(m.PermissionGroups) > 0 {
		policy++
	}
	if version > 9007199254740991 || policy > 9007199254740991 {
		return empty, applications.ErrPolicyConflict
	}
	members, grants, e := insertManifestInTx(ctx, tx, p, m, appschema.Limits{LockTimeout: 2 * time.Second, StatementTimeout: 10 * time.Second})
	if e != nil {
		return empty, e
	}
	if in.Source.Kind == "directory" && in.Target.DirectoryID != nil {
		if _, e = tx.Exec(ctx, "UPDATE applications.directories SET parent_id=$1 WHERE app_id=$2 AND id=$3", *in.Target.DirectoryID, w.Target.ID, root); e != nil {
			return empty, e
		}
	}
	if target != "" {
		if _, e = tx.Exec(ctx, "UPDATE applications.apps SET structure_version=$2,policy_revision=$3 WHERE id=$1", target, version, policy); e != nil {
			return empty, e
		}
	}
	var summary []byte
	event, reason, object := "application_structure_changed", "STRUCTURE_COPY", in.Source.Kind
	if target == "" {
		event, reason, object = "application_changed", "APPLICATION_CREATED", "application"
		summary, e = json.Marshal(map[string]any{"appId": w.Target.ID, "operationId": operationID, "beforePolicyRevision": 0, "afterPolicyRevision": 1, "changeCounts": map[string]int{"applications": 1, "groups": len(m.PermissionGroups), "members": members, "grants": grants}})
	} else {
		schema, view := 0, 0
		if in.Source.Kind == "table" || in.Source.Kind == "form" {
			schema = 1
		}
		if in.Source.Kind == "form" {
			view = 1
		}
		count := len(m.Directories) + len(m.Tables) + len(m.Forms) + len(m.Workflows) + len(m.PermissionGroups)
		for _, t := range m.Tables {
			count += len(t.Fields)
		}
		summary, e = json.Marshal(map[string]any{"appId": w.Target.ID, "operationId": operationID, "structureVersion": version, "schemaVersion": schema, "viewVersion": view, "changeCount": count})
	}
	if e != nil {
		return empty, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO auth.authentication_events(event_type,outcome,actor_user_id,session_ref,reason_code,request_id,client_ip,user_agent,object_type,object_id,change_summary) VALUES($1,'success',$2,NULLIF($3,'')::uuid,$4,$5,NULLIF($6,'')::inet,NULLIF($7,''),$8,$9,$10::jsonb)`, event, p.UserID, p.SessionRef, reason, metadata.RequestID, metadata.ClientIP, metadata.UserAgent, object, root, summary); e != nil {
		return empty, e
	}
	data, e := json.Marshal(map[string]any{"operationId": operationID, "appId": w.Target.ID, "resourceKind": in.Source.Kind, "id": root, "structureVersion": version, "policyRevision": policy, "status": "copied"})
	if e != nil {
		return empty, e
	}
	location := "/api/v1/applications/" + w.Target.ID
	if target != "" {
		location += "/" + map[string]string{"directory": "directories", "table": "tables", "form": "forms"}[in.Source.Kind] + "/" + root
	}
	result := applications.Result{Status: 201, Location: location, Data: data}
	if e = w.Complete(ctx, operationID, result); e != nil {
		return empty, e
	}
	if e = w.Commit(ctx); e != nil {
		return empty, e
	}
	return result, nil
}
