package devspace

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

type MergeConflict struct {
	Entity string
	Key    string
	Field  string
	Base   string
	Ours   string
	Theirs string
}

func mergeManifests(base, ours, theirs Manifest) (Manifest, []MergeConflict, error) {
	if err := ValidateManifest(base); err != nil {
		return Manifest{}, nil, fmt.Errorf("base manifest failed validation: %w", err)
	}
	if err := ValidateManifest(ours); err != nil {
		return Manifest{}, nil, fmt.Errorf("ours manifest failed validation: %w", err)
	}
	if err := ValidateManifest(theirs); err != nil {
		return Manifest{}, nil, fmt.Errorf("theirs manifest failed validation: %w", err)
	}

	merged := ours
	merged.Projects = nil
	merged.Access = nil

	var conflicts []MergeConflict

	projects, projectConflicts := mergeProjectRecords(base.Projects, ours.Projects, theirs.Projects)
	conflicts = append(conflicts, projectConflicts...)
	merged.Projects = projects

	access, accessConflicts := mergeAccessRecords(base.Access, ours.Access, theirs.Access)
	conflicts = append(conflicts, accessConflicts...)
	merged.Access = access

	if len(conflicts) > 0 {
		return merged, conflicts, nil
	}
	if err := ValidateManifest(merged); err != nil {
		return merged, nil, fmt.Errorf("merged manifest failed validation: %w", err)
	}
	return merged, nil, nil
}

func mergeProjectRecords(base, ours, theirs []Project) ([]Project, []MergeConflict) {
	baseByID := projectByID(base)
	oursByID := projectByID(ours)
	theirsByID := projectByID(theirs)

	ids := map[string]bool{}
	for id := range baseByID {
		ids[id] = true
	}
	for id := range oursByID {
		ids[id] = true
	}
	for id := range theirsByID {
		ids[id] = true
	}

	var merged []Project
	var conflicts []MergeConflict
	for _, id := range sortedKeys(ids) {
		baseProject, inBase := baseByID[id]
		oursProject, inOurs := oursByID[id]
		theirsProject, inTheirs := theirsByID[id]

		project, ok, conflict := mergeProjectRecord(id, baseProject, inBase, oursProject, inOurs, theirsProject, inTheirs)
		if conflict != nil {
			conflicts = append(conflicts, *conflict)
		}
		if ok {
			merged = append(merged, project)
		}
	}

	slices.SortFunc(merged, func(a, b Project) int {
		return strings.Compare(a.Path, b.Path)
	})
	return merged, conflicts
}

func mergeProjectRecord(id string, base Project, inBase bool, ours Project, inOurs bool, theirs Project, inTheirs bool) (Project, bool, *MergeConflict) {
	return mergeThreeWay("project", id, base, inBase, ours, inOurs, theirs, inTheirs)
}

func projectByID(projects []Project) map[string]Project {
	byID := make(map[string]Project, len(projects))
	for _, p := range projects {
		byID[p.ID] = p
	}
	return byID
}

func mergeAccessRecords(base, ours, theirs []ProjectAccess) ([]ProjectAccess, []MergeConflict) {
	baseByKey := accessByKey(base)
	oursByKey := accessByKey(ours)
	theirsByKey := accessByKey(theirs)

	keys := map[string]bool{}
	for key := range baseByKey {
		keys[key] = true
	}
	for key := range oursByKey {
		keys[key] = true
	}
	for key := range theirsByKey {
		keys[key] = true
	}

	var merged []ProjectAccess
	var conflicts []MergeConflict
	for _, key := range sortedKeys(keys) {
		baseAccess, inBase := baseByKey[key]
		oursAccess, inOurs := oursByKey[key]
		theirsAccess, inTheirs := theirsByKey[key]

		access, ok, conflict := mergeAccessRecord(key, baseAccess, inBase, oursAccess, inOurs, theirsAccess, inTheirs)
		if conflict != nil {
			conflicts = append(conflicts, *conflict)
		}
		if ok {
			merged = append(merged, access)
		}
	}

	slices.SortFunc(merged, func(a, b ProjectAccess) int {
		return strings.Compare(accessKey(a), accessKey(b))
	})
	return merged, conflicts
}

func mergeAccessRecord(key string, base ProjectAccess, inBase bool, ours ProjectAccess, inOurs bool, theirs ProjectAccess, inTheirs bool) (ProjectAccess, bool, *MergeConflict) {
	return mergeThreeWay("access", key, base, inBase, ours, inOurs, theirs, inTheirs)
}

func mergeThreeWay[T any](entity, key string, base T, inBase bool, ours T, inOurs bool, theirs T, inTheirs bool) (T, bool, *MergeConflict) {
	var zero T
	switch {
	case !inBase:
		switch {
		case inOurs && inTheirs:
			if reflect.DeepEqual(ours, theirs) {
				return ours, true, nil
			}
			return ours, true, &MergeConflict{Entity: entity, Key: key, Field: "*", Ours: fmt.Sprintf("%+v", ours), Theirs: fmt.Sprintf("%+v", theirs)}
		case inOurs:
			return ours, true, nil
		case inTheirs:
			return theirs, true, nil
		default:
			return zero, false, nil
		}
	case inOurs && inTheirs:
		oursChanged := !reflect.DeepEqual(base, ours)
		theirsChanged := !reflect.DeepEqual(base, theirs)
		switch {
		case !oursChanged && !theirsChanged:
			return base, true, nil
		case oursChanged && !theirsChanged:
			return ours, true, nil
		case !oursChanged && theirsChanged:
			return theirs, true, nil
		case reflect.DeepEqual(ours, theirs):
			return ours, true, nil
		default:
			return ours, true, &MergeConflict{Entity: entity, Key: key, Field: "*", Base: fmt.Sprintf("%+v", base), Ours: fmt.Sprintf("%+v", ours), Theirs: fmt.Sprintf("%+v", theirs)}
		}
	case inOurs:
		return ours, true, nil
	case inTheirs:
		return theirs, true, nil
	default:
		return zero, false, nil
	}
}

func sortedKeys(keys map[string]bool) []string {
	out := make([]string, 0, len(keys))
	for key := range keys {
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}

func accessByKey(access []ProjectAccess) map[string]ProjectAccess {
	byKey := make(map[string]ProjectAccess, len(access))
	for _, a := range access {
		byKey[accessKey(a)] = a
	}
	return byKey
}

func accessKey(access ProjectAccess) string {
	return access.ProjectID + "\x00" + access.UserID + "\x00" + access.TeamID
}
