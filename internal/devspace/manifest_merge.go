package devspace

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

type MergeConflict struct {
	Entity string `json:"entity"`
	Key    string `json:"key"`
	Field  string `json:"field"`
	Base   string `json:"base,omitempty"`
	Ours   string `json:"ours"`
	Theirs string `json:"theirs"`
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
	merged.Users = nil
	merged.Teams = nil

	var conflicts []MergeConflict

	projects, projectConflicts := mergeProjectRecords(base.Projects, ours.Projects, theirs.Projects)
	conflicts = append(conflicts, projectConflicts...)
	merged.Projects = projects

	access, accessConflicts := mergeAccessRecords(base.Access, ours.Access, theirs.Access)
	conflicts = append(conflicts, accessConflicts...)
	merged.Access = access

	// ponytail: users/teams merge at whole-record granularity (one side wins per
	// record); upgrade to per-field merging inside mergeThreeWay if concurrent
	// edits to the same user/team record become common.
	users, userConflicts := mergeRecordSection("user", base.Users, ours.Users, theirs.Users, userID)
	conflicts = append(conflicts, userConflicts...)
	merged.Users = users

	teams, teamConflicts := mergeRecordSection("team", base.Teams, ours.Teams, theirs.Teams, teamID)
	conflicts = append(conflicts, teamConflicts...)
	merged.Teams = teams

	if len(conflicts) > 0 {
		return merged, conflicts, nil
	}
	if err := ValidateManifest(merged); err != nil {
		return merged, nil, fmt.Errorf("merged manifest failed validation: %w", err)
	}
	return merged, nil, nil
}

// Projects reconcile by Path: paths are what humans review and what scan derives
// IDs from, so the same folder created on two machines merges as one project.
// A same-path pair with different IDs is surfaced as an "id" conflict instead of
// dead-ending in ValidateManifest's duplicate-path error.
func mergeProjectRecords(base, ours, theirs []Project) ([]Project, []MergeConflict) {
	merged, conflicts := mergeRecordSection("project", base, ours, theirs, projectPath)
	oursByPath := projectByPath(ours)
	theirsByPath := projectByPath(theirs)
	for i, conflict := range conflicts {
		oursProject, inOurs := oursByPath[conflict.Key]
		theirsProject, inTheirs := theirsByPath[conflict.Key]
		if inOurs && inTheirs && oursProject.ID != theirsProject.ID {
			conflicts[i].Field = "id"
		}
	}
	return merged, conflicts
}

func projectByPath(projects []Project) map[string]Project {
	return recordsByKey(projects, projectPath)
}

func projectPath(p Project) string { return p.Path }

func userID(u User) string { return u.ID }

func teamID(t Team) string { return t.ID }

func mergeAccessRecords(base, ours, theirs []ProjectAccess) ([]ProjectAccess, []MergeConflict) {
	return mergeRecordSection("access", base, ours, theirs, accessKey)
}

func mergeRecordSection[T any](entity string, base, ours, theirs []T, key func(T) string) ([]T, []MergeConflict) {
	baseByKey := recordsByKey(base, key)
	oursByKey := recordsByKey(ours, key)
	theirsByKey := recordsByKey(theirs, key)

	keys := map[string]bool{}
	for k := range baseByKey {
		keys[k] = true
	}
	for k := range oursByKey {
		keys[k] = true
	}
	for k := range theirsByKey {
		keys[k] = true
	}

	var merged []T
	var conflicts []MergeConflict
	for _, k := range sortedKeys(keys) {
		baseRecord, inBase := baseByKey[k]
		oursRecord, inOurs := oursByKey[k]
		theirsRecord, inTheirs := theirsByKey[k]

		record, ok, conflict := mergeThreeWay(entity, k, baseRecord, inBase, oursRecord, inOurs, theirsRecord, inTheirs)
		if conflict != nil {
			conflicts = append(conflicts, *conflict)
		}
		if ok {
			merged = append(merged, record)
		}
	}

	slices.SortFunc(merged, func(a, b T) int {
		return strings.Compare(key(a), key(b))
	})
	return merged, conflicts
}

func recordsByKey[T any](records []T, key func(T) string) map[string]T {
	byKey := make(map[string]T, len(records))
	for _, record := range records {
		byKey[key(record)] = record
	}
	return byKey
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
		if reflect.DeepEqual(base, ours) {
			return zero, false, nil
		}
		return ours, true, &MergeConflict{Entity: entity, Key: key, Field: "*", Base: fmt.Sprintf("%+v", base), Ours: fmt.Sprintf("%+v", ours), Theirs: "<deleted>"}
	case inTheirs:
		if reflect.DeepEqual(base, theirs) {
			return zero, false, nil
		}
		return theirs, true, &MergeConflict{Entity: entity, Key: key, Field: "*", Base: fmt.Sprintf("%+v", base), Ours: "<deleted>", Theirs: fmt.Sprintf("%+v", theirs)}
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
