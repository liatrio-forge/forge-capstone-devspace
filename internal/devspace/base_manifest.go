package devspace

var recordBaseManifestForSync = recordBaseManifest

func recordBaseManifest(m Manifest) error {
	path, err := baseManifestPath()
	if err != nil {
		return err
	}
	return writeJSON(path, m, 0o600)
}

func loadBaseManifest() (Manifest, bool, error) {
	var m Manifest
	path, err := baseManifestPath()
	if err != nil {
		return Manifest{}, false, err
	}
	if err := readJSON(path, &m); err != nil {
		if missing(err) {
			return Manifest{}, false, nil
		}
		return Manifest{}, false, err
	}
	return m, true, nil
}

// The sync mutation has already succeeded by the time this runs, so a snapshot
// write failure must not turn the completed push/pull into a reported failure.
func recordBaseManifestAfterSync(m Manifest) {
	_ = recordBaseManifestForSync(m)
}
