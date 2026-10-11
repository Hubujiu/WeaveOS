package apptemplates

type CopyGroupMapping struct {
	SourceGroupID string `json:"sourceGroupId"`
	Name          string `json:"name"`
}

func MapCopyGroups(in Manifest, mappings []CopyGroupMapping) (Manifest, error) {
	return Manifest{}, ErrInvalid
}
