package devspace

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
