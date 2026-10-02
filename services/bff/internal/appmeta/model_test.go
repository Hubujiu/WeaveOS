package appmeta_test

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"testing"

	"github.com/Hubujiu/WeaveOS/services/bff/internal/appmeta"
)

// Oracles: PRD AP-FR-01/02 and the 2026-10-02 B1 PLAN; ADR009 D01.
// These deliberately use opaque IDs rather than assuming an unapproved UUID
// version, ordering rule, root policy, name uniqueness, or directory limit.
func sample() appmeta.Structure {
	return appmeta.Structure{
		Application: appmeta.Application{ID: "app-a", Name: "Expense"},
		Groups: []appmeta.Group{
			{ID: "g-root", ApplicationID: "app-a", Name: "Requests"},
			{ID: "g-child", ApplicationID: "app-a", ParentID: "g-root", Name: "Expense"},
			{ID: "g-leaf", ApplicationID: "app-a", ParentID: "g-child", Name: "Expense"},
		},
		Tables: []appmeta.TableDefinition{{ID: "table-expense", ApplicationID: "app-a", GroupID: "g-child", Name: "Expense"}},
		Views: []appmeta.View{
			{ID: "view-1", ApplicationID: "app-a", TableID: "table-expense", GroupID: "g-leaf", Name: "Expense"},
			{ID: "view-2", ApplicationID: "app-a", TableID: "table-expense", GroupID: "g-root", Name: "Expense"},
		},
	}
}

func requireInvalid(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, appmeta.ErrInvalid) {
		t.Fatalf("invalid metadata must be rejected: got %v", err)
	}
}

func TestStructureUsesStableReferencesAcrossRenameAndReordering(t *testing.T) {
	s := sample()
	s.Application.Name = "Renamed app"
	s.Tables[0].Name = "Renamed table"
	s.Groups[0].Name = "Renamed directory"
	s.Groups[0], s.Groups[2] = s.Groups[2], s.Groups[0]
	s.Views[0], s.Views[1] = s.Views[1], s.Views[0]
	before := sample()
	if err := appmeta.ValidateStructure(s); err != nil {
		t.Fatal(err)
	}
	if s.Application.ID != before.Application.ID || s.Tables[0].ID != before.Tables[0].ID || s.Views[0].TableID != "table-expense" {
		t.Fatal("renames/reordering changed stable identity or shared-table reference")
	}
}

func TestStructureRejectsCyclesAndForeignOrMissingReferences(t *testing.T) {
	cases := []struct {
		name   string
		change func(*appmeta.Structure)
	}{
		{"missing application ID", func(s *appmeta.Structure) { s.Application.ID = "" }},
		{"missing group ID", func(s *appmeta.Structure) { s.Groups[0].ID = "" }},
		{"duplicate group ID", func(s *appmeta.Structure) { s.Groups[2].ID = s.Groups[0].ID }},
		{"foreign group owner", func(s *appmeta.Structure) { s.Groups[0].ApplicationID = "app-b" }},
		{"foreign parent owner", func(s *appmeta.Structure) { s.Groups[1].ApplicationID = "app-b" }},
		{"absent parent", func(s *appmeta.Structure) { s.Groups[2].ParentID = "missing" }},
		{"self cycle", func(s *appmeta.Structure) { s.Groups[1].ParentID = "g-child" }},
		{"three node cycle", func(s *appmeta.Structure) { s.Groups[0].ParentID = "g-leaf" }},
		{"disconnected cycle", func(s *appmeta.Structure) {
			s.Groups = append(s.Groups, appmeta.Group{ID: "x", ApplicationID: "app-a", ParentID: "y"}, appmeta.Group{ID: "y", ApplicationID: "app-a", ParentID: "x"})
		}},
		{"missing table ID", func(s *appmeta.Structure) { s.Tables[0].ID = "" }},
		{"duplicate table ID", func(s *appmeta.Structure) { s.Tables = append(s.Tables, s.Tables[0]) }},
		{"foreign table owner", func(s *appmeta.Structure) { s.Tables[0].ApplicationID = "app-b" }},
		{"missing table directory", func(s *appmeta.Structure) { s.Tables[0].GroupID = "missing" }},
		{"missing view ID", func(s *appmeta.Structure) { s.Views[0].ID = "" }},
		{"duplicate view ID", func(s *appmeta.Structure) { s.Views[1].ID = s.Views[0].ID }},
		{"foreign view owner", func(s *appmeta.Structure) { s.Views[0].ApplicationID = "app-b" }},
		{"missing view directory", func(s *appmeta.Structure) { s.Views[0].GroupID = "missing" }},
		{"missing view table", func(s *appmeta.Structure) { s.Views[0].TableID = "missing" }},
		{"empty view table", func(s *appmeta.Structure) { s.Views[0].TableID = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { s := sample(); tc.change(&s); requireInvalid(t, appmeta.ValidateStructure(s)) })
	}
}

func TestStructureAllowsEmptyApplicationAndValidDeepTree(t *testing.T) {
	empty := appmeta.Structure{Application: appmeta.Application{ID: "empty"}}
	if err := appmeta.ValidateStructure(empty); err != nil {
		t.Fatal(err)
	}
	s := empty
	// This exercises iterative traversal; it does not set a product depth quota.
	for i := 0; i < 50000; i++ {
		id := appmeta.ID(strconv.Itoa(i))
		var parent appmeta.ID
		if i > 0 {
			parent = s.Groups[i-1].ID
		}
		s.Groups = append(s.Groups, appmeta.Group{ID: id, ApplicationID: "empty", ParentID: parent})
	}
	if err := appmeta.ValidateStructure(s); err != nil {
		t.Fatal(err)
	}
	if err := appmeta.ValidateStructure(appmeta.Structure{Application: appmeta.Application{ID: "root-level"}, Tables: []appmeta.TableDefinition{{ID: "table", ApplicationID: "root-level"}}, Views: []appmeta.View{{ID: "view", ApplicationID: "root-level", TableID: "table"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogValidatesApplicationGroupReferences(t *testing.T) {
	valid := func() appmeta.Catalog {
		return appmeta.Catalog{Applications: []appmeta.Application{{ID: "app-a"}, {ID: "app-b"}}, Groups: []appmeta.ApplicationGroup{{ID: "catalog-g"}}, Links: []appmeta.ApplicationGroupLink{{ApplicationID: "app-a", GroupID: "catalog-g"}, {ApplicationID: "app-b", GroupID: "catalog-g"}}}
	}
	if err := appmeta.ValidateCatalog(valid()); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		change func(*appmeta.Catalog)
	}{
		{"missing app ID", func(c *appmeta.Catalog) { c.Applications[0].ID = "" }},
		{"duplicate app ID", func(c *appmeta.Catalog) { c.Applications[1].ID = "app-a" }},
		{"missing group ID", func(c *appmeta.Catalog) { c.Groups[0].ID = "" }},
		{"duplicate group ID", func(c *appmeta.Catalog) { c.Groups = append(c.Groups, c.Groups[0]) }},
		{"missing app", func(c *appmeta.Catalog) { c.Links[0].ApplicationID = "missing" }},
		{"missing group", func(c *appmeta.Catalog) { c.Links[0].GroupID = "missing" }},
		{"duplicate link", func(c *appmeta.Catalog) { c.Links = append(c.Links, c.Links[0]) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { c := valid(); tc.change(&c); requireInvalid(t, appmeta.ValidateCatalog(c)) })
	}
}

type readerFunc func(context.Context, appmeta.ID) (appmeta.Structure, error)

func (f readerFunc) LoadStructure(ctx context.Context, id appmeta.ID) (appmeta.Structure, error) {
	return f(ctx, id)
}

func TestReaderValidatesActualStoredOwnerAndAllReferences(t *testing.T) {
	reader := readerFunc(func(_ context.Context, id appmeta.ID) (appmeta.Structure, error) {
		if id != "app-a" {
			t.Fatal("wrong requested ID")
		}
		return sample(), nil
	})
	got, err := appmeta.ReadStructure(context.Background(), reader, "app-a")
	if err != nil || !reflect.DeepEqual(got, sample()) {
		t.Fatalf("coherent snapshot: %v", err)
	}
	for _, foreign := range []bool{false, true} {
		r := readerFunc(func(context.Context, appmeta.ID) (appmeta.Structure, error) {
			s := sample()
			if foreign {
				s.Application.ID = "app-b"
			} else {
				s.Groups[0].ParentID = "g-leaf"
			}
			return s, nil
		})
		got, err := appmeta.ReadStructure(context.Background(), r, "app-a")
		requireInvalid(t, err)
		if !reflect.DeepEqual(got, appmeta.Structure{}) {
			t.Fatal("invalid stored snapshot leaked to caller")
		}
	}
}

func TestReaderPropagatesStorageErrorWithoutPartialSnapshot(t *testing.T) {
	failed := errors.New("storage failed")
	r := readerFunc(func(context.Context, appmeta.ID) (appmeta.Structure, error) { return sample(), failed })
	got, err := appmeta.ReadStructure(context.Background(), r, "app-a")
	if !errors.Is(err, failed) || !reflect.DeepEqual(got, appmeta.Structure{}) {
		t.Fatalf("storage error must return no snapshot: %v", err)
	}
}

func TestReaderRejectsInvalidInputAndCancellationBeforeStorage(t *testing.T) {
	r := readerFunc(func(context.Context, appmeta.ID) (appmeta.Structure, error) {
		t.Fatal("invalid/canceled reads must not call storage")
		return sample(), nil
	})
	_, err := appmeta.ReadStructure(context.Background(), nil, "app-a")
	requireInvalid(t, err)
	_, err = appmeta.ReadStructure(context.Background(), r, "")
	requireInvalid(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = appmeta.ReadStructure(ctx, r, "app-a")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read: %v", err)
	}
}
