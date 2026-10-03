// Package apppolicy evaluates a caller-supplied, coherent authorization snapshot.
// It authenticates nobody, loads no grants, and performs no business operation.
package apppolicy

// TrustedActor is injected by an authenticated server-side caller. None of its
// fields may be populated from client claims. IDs are opaque stable identifiers.
type TrustedActor struct {
	ID             string
	BootstrapAdmin bool
	CreateApp      bool
}

type Application struct {
	ID      string
	OwnerID string
	Exists  bool
}

// ResourceRef includes the caller-owned kind namespace: equal IDs in different
// kinds or applications are distinct resources. No parent inheritance occurs.
type ResourceRef struct {
	ApplicationID string
	Kind          string
	ID            string
}

// Resource existence and actual ownership must be resolved by the trusted caller.
type Resource struct {
	Ref    ResourceRef
	Exists bool
}

// RowFact belongs to the already-resolved target resource. The caller must
// verify that association. CreatedBy is the immutable author, not LastEditor.
type RowFact struct {
	ApplicationID string
	Exists        bool
	CreatedBy     string
	LastEditor    string
}

// These internal primitives are not the product's complete action registry.
// Approval eligibility and screenshot actions are deliberately not modeled.
type Action string

const (
	MenuEnter Action = "menu.enter"
	DataRead  Action = "data.read"
	DataEdit  Action = "data.edit"
)

type RowScope string

const (
	AllRows RowScope = "all"
	OwnRows RowScope = "own"
)

// Grant is one indivisible resource/action/row/field tuple. Fields belong only
// to this action. Empty data masks grant no fields; menu grants use AllRows and
// an empty mask. There are no wildcard fields, deny rules or descendant scopes.
type Grant struct {
	Resource ResourceRef
	Action   Action
	Rows     RowScope
	Fields   []string
}

type PermissionGroup struct {
	ID            string
	ApplicationID string
	Enabled       bool
	MemberIDs     []string
	Grants        []Grant
}

// TrustedContext contains one coherent policy snapshot for one application.
// Loading, revision/CAS, revocation, and snapshot authenticity belong to callers.
type TrustedContext struct {
	Actor       TrustedActor
	Application Application
	Groups      []PermissionGroup
}
