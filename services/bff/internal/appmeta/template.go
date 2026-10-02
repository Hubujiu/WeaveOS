package appmeta

import (
	"errors"
	"slices"
)

// TemplateSection is untrusted component content retained by an upstream parser.
// B1 must not discard unrecognized parts or accept opaque component payloads.
// The actual manifest/archive parser and owner component schemas are not frozen.
type TemplateSection struct {
	Kind    string
	Payload []byte
}

// BindingReference is a symbolic source reference, never a copied live grant.
// The binding-kind registry and target authorization/resolution belong to root.
type BindingReference struct {
	Kind     string
	SourceID ID
}

// WorkflowReference carries only stable ownership references, not executable
// node/trigger configuration. Enabled is the source state, never import state.
type WorkflowReference struct {
	ID      ID
	TableID ID
	Enabled bool
}

// StructureTemplate is the B1 metadata projection, not a published manifest or
// a complete template format. Records/attachments/secrets have no typed slot.
// Any component payload must remain in Sections and fail closed until its
// owner supplies the reviewed contract through root's integration work.
type StructureTemplate struct {
	Structure Structure
	Workflows []WorkflowReference
	Bindings  []BindingReference
	Sections  []TemplateSection
}

var ErrTemplateContractRequired = errors.New("template component contract is not frozen")

type ImportedWorkflow struct {
	ID      ID
	TableID ID
	Enabled bool
}

// ImportPreview is non-executable and performs no storage writes. The snapshots
// returned by accessors cannot mutate its pending bindings/disabled workflows.
// ID remapping, binding and actual import remain separate frozen-contract work.
type ImportPreview struct {
	structure Structure
	workflows []ImportedWorkflow
	unbound   []BindingReference
}

func ValidateStructureTemplate(candidate StructureTemplate) error {
	if err := ValidateStructure(candidate.Structure); err != nil {
		return err
	}
	tables := make(map[ID]bool, len(candidate.Structure.Tables))
	for _, table := range candidate.Structure.Tables {
		tables[table.ID] = true
	}
	workflows, err := index(candidate.Workflows, func(w WorkflowReference) ID { return w.ID }, "workflow")
	if err != nil {
		return err
	}
	for _, workflow := range workflows {
		if !tables[workflow.TableID] {
			return invalid("template workflow table is absent")
		}
	}
	bindings := make(map[BindingReference]bool, len(candidate.Bindings))
	for _, reference := range candidate.Bindings {
		if reference.Kind == "" || reference.SourceID == "" {
			return invalid("incomplete symbolic binding reference")
		}
		if bindings[reference] {
			return invalid("duplicate symbolic binding reference")
		}
		bindings[reference] = true
	}
	for _, section := range candidate.Sections {
		switch section.Kind {
		case "fields", "layouts", "workflows", "roles":
			// Arbitrary payloads stay rejected until root integrates the
			// component owner's reviewed schema, even in an allowed section.
		default:
			return invalid("unknown or forbidden template section")
		}
	}
	if len(candidate.Sections) != 0 {
		return ErrTemplateContractRequired
	}
	return nil
}

// PreviewStructureImport never binds source accounts, expands grants, generates
// replacement IDs, writes metadata, or activates the source workflows.
func PreviewStructureImport(candidate StructureTemplate) (ImportPreview, error) {
	if err := ValidateStructureTemplate(candidate); err != nil {
		return ImportPreview{}, err
	}
	preview := ImportPreview{
		structure: cloneStructure(candidate.Structure),
		unbound:   slices.Clone(candidate.Bindings),
		workflows: make([]ImportedWorkflow, len(candidate.Workflows)),
	}
	for i, workflow := range candidate.Workflows {
		preview.workflows[i] = ImportedWorkflow{ID: workflow.ID, TableID: workflow.TableID, Enabled: false}
	}
	return preview, nil
}

func cloneStructure(s Structure) Structure {
	s.Groups = slices.Clone(s.Groups)
	s.Tables = slices.Clone(s.Tables)
	s.Views = slices.Clone(s.Views)
	return s
}

func (p ImportPreview) Structure() Structure                  { return cloneStructure(p.structure) }
func (p ImportPreview) Workflows() []ImportedWorkflow         { return slices.Clone(p.workflows) }
func (p ImportPreview) UnboundReferences() []BindingReference { return slices.Clone(p.unbound) }
