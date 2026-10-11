package apptemplates

type CopyGroupMapping struct {
	SourceGroupID string `json:"sourceGroupId"`
	Name          string `json:"name"`
}

func MapCopyGroups(in Manifest, mappings []CopyGroupMapping) (Manifest, error) {
	m, err := NormalizeManifest(in)
	if err != nil || len(mappings) != len(m.PermissionGroups) {
		return Manifest{}, ErrInvalid
	}
	wanted := make(map[string]bool, len(m.PermissionGroups))
	for _, g := range m.PermissionGroups {
		wanted[g.ID] = true
	}
	names := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		normalized, ok := name(mapping.Name)
		if !wanted[mapping.SourceGroupID] || names[mapping.SourceGroupID] != "" || !ok {
			return Manifest{}, ErrInvalid
		}
		names[mapping.SourceGroupID] = normalized
	}
	for i := range m.PermissionGroups {
		m.PermissionGroups[i].Name = names[m.PermissionGroups[i].ID]
	}
	return m, nil
}
