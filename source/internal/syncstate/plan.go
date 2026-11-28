package syncstate

import (
	"errors"
	"sort"
)

const maxPlanEntries = 4096

// Plan returns remote winners sorted by name without mutating either input.
// An identical remote version is omitted.
func Plan(local, remote []Versioned) ([]Versioned, error) {
	if len(local) > maxPlanEntries || len(remote) > maxPlanEntries {
		return nil, errors.New("metadata plan exceeds entry limit")
	}

	localByName := make(map[string]Versioned, len(local))
	for _, item := range local {
		if err := validateVersioned(item); err != nil {
			return nil, err
		}
		if _, exists := localByName[item.Name]; exists {
			return nil, errors.New("duplicate local file name")
		}
		localByName[item.Name] = item
	}

	seenRemote := make(map[string]bool, len(remote))
	plan := make([]Versioned, 0)

	for _, item := range remote {
		if err := validateVersioned(item); err != nil {
			return nil, err
		}
		if seenRemote[item.Name] {
			return nil, errors.New("duplicate remote file name")
		}
		seenRemote[item.Name] = true

		current, exists := localByName[item.Name]
		if !exists {
			plan = append(plan, item)
			continue
		}

		order, err := Compare(item, current)
		if err != nil {
			return nil, err
		}
		if order > 0 {
			plan = append(plan, item)
		}
	}

	sort.Slice(plan, func(i, j int) bool {
		return plan[i].Name < plan[j].Name
	})

	return plan, nil
}
