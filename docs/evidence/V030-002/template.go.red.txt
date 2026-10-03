package appmeta

import "errors"

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

func ValidateStructureTemplate(StructureTemplate) error               { return nil }
func PreviewStructureImport(StructureTemplate) (ImportPreview, error) { return ImportPreview{}, nil }
func (p ImportPreview) Structure() Structure                          { return p.structure }
func (p ImportPreview) Workflows() []ImportedWorkflow                 { return p.workflows }
func (p ImportPreview) UnboundReferences() []BindingReference         { return p.unbound }
