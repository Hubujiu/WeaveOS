// Package appmeta contains application metadata invariants. It does not expose
// HTTP routes, grant permissions, create physical tables, or execute workflows.
package appmeta

import (
	"context"
	"errors"
)

// ID is an opaque stable reference. Its generation/encoding belongs to the
// integration contract; display names and slice positions are never IDs.
type ID string

type Application struct {
	ID   ID
	Name string
}

type ApplicationGroup struct {
	ID   ID
	Name string
}

type ApplicationGroupLink struct {
	ApplicationID ID
	GroupID       ID
}

// Catalog expresses references, not a product decision about group membership
// cardinality, ordering, permissions, or application-group nesting.
type Catalog struct {
	Applications []Application
	Groups       []ApplicationGroup
	Links        []ApplicationGroupLink
}

type Group struct {
	ID            ID
	ApplicationID ID
	ParentID      ID // empty represents no parent in this in-process model
	Name          string
}

// TableDefinition has no records or physical-table names. B2 owns fields/DDL.
type TableDefinition struct {
	ID            ID
	ApplicationID ID
	GroupID       ID
	Name          string
}

// View references a logical table. Layout/type schemas remain integration work.
type View struct {
	ID            ID
	ApplicationID ID
	TableID       ID
	GroupID       ID
	Name          string
}

type Structure struct {
	Application Application
	Groups      []Group
	Tables      []TableDefinition
	Views       []View
}

var ErrInvalid = errors.New("invalid application metadata")

// Reader must return one coherent application metadata snapshot. This contract
// is read-only; write transactions/CAS/migrations require root's frozen schema.
// The caller must authorize against the actual resource owner before use.
type Reader interface {
	LoadStructure(context.Context, ID) (Structure, error)
}

// No-behavior declarations allow tests to reach invariant assertions during RED.
func ValidateCatalog(Catalog) error     { return nil }
func ValidateStructure(Structure) error { return nil }
func ReadStructure(ctx context.Context, reader Reader, appID ID) (Structure, error) {
	if reader == nil {
		return Structure{}, nil
	}
	return reader.LoadStructure(ctx, appID)
}
