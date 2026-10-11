package apptemplates

// SelectCopyManifest selects approved structure and its required dependencies.
// Initial declaration only: behavior is intentionally absent for RED evidence.
func SelectCopyManifest(in Manifest, kind, id string) (Manifest, error) {
	return Manifest{}, ErrInvalid
}
