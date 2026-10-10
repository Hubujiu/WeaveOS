package apptemplates

type References struct {
	UserIDs       []string `json:"userIds"`
	DepartmentIDs []string `json:"departmentIds"`
}
type Binding struct {
	Kind     string `json:"kind"`
	SourceID string `json:"sourceId"`
	TargetID string `json:"targetId"`
}

func ExternalReferences(Manifest) (References, error)             { return References{}, ErrInvalid }
func MapExternalReferences(Manifest, []Binding) (Manifest, error) { return Manifest{}, ErrInvalid }
