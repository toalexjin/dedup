// File deduplication
package aa

import (
	"errors"
	"os"
	"strings"
)

var E_INVALID_POLICY = errors.New("Invalid command line policy argument.")

const (
	// The first file should be removed.
	DELETE_WHICH_FIRST = iota

	// The second file should be removed.
	DELETE_WHICH_SECOND = iota

	// Either could be removed.
	DELETE_WHICH_EITHER = iota

	// Neither could be removed.
	DELETE_WHICH_NEITHER = iota
)

const (
	// -1 means old and 1 means new.
	POLICY_CATEGORY_MOD_TIME = iota

	// -1 means short name and 1 means long name.
	POLICY_CATEGORY_NAME = iota

	// -1 means short path and 1 means long path.
	POLICY_CATEGORY_PATH = iota
)

// Number of policy categories.
const POLICY_CATEGORY_COUNT = 3

// Policy item mapping table.
var policyItemMapping = map[string]*policyItem{
	"old":       &policyItem{category: POLICY_CATEGORY_MOD_TIME, value: -1},
	"new":       &policyItem{category: POLICY_CATEGORY_MOD_TIME, value: 1},
	"shortname": &policyItem{category: POLICY_CATEGORY_NAME, value: -1},
	"longname":  &policyItem{category: POLICY_CATEGORY_NAME, value: 1},
	"shortpath": &policyItem{category: POLICY_CATEGORY_PATH, value: -1},
	"longpath":  &policyItem{category: POLICY_CATEGORY_PATH, value: 1},
}

// Default policy.
var defaultPolicyItems = []*policyItem{
	&policyItem{category: POLICY_CATEGORY_MOD_TIME, value: 1},
	&policyItem{category: POLICY_CATEGORY_NAME, value: 1},
	&policyItem{category: POLICY_CATEGORY_PATH, value: 1},
}

// Policy interface.
type Policy interface {
	// Check which file should be removed.
	//
	// Return value is DELETE_WHICH_???
	DeleteWhich(first, second *FileAttr) int
}

// Policy item.
type policyItem struct {
	category int
	value    int
}

// Policy implementation.
type policyImpl struct {
	items []*policyItem
}

func (me *policyImpl) DeleteWhich(first, second *FileAttr) int {
	// Check if the two paths are identical.
	if os.PathSeparator == '/' {
		if first.Path == second.Path {
			return DELETE_WHICH_NEITHER
		}
	} else {
		if strings.EqualFold(first.Path, second.Path) {
			return DELETE_WHICH_NEITHER
		}
	}

	for _, item := range me.items {
		switch item.category {
		case POLICY_CATEGORY_MOD_TIME:
			if first.ModTime != second.ModTime {
				if item.value < 0 && first.ModTime < second.ModTime {
					return DELETE_WHICH_FIRST
				} else {
					return DELETE_WHICH_SECOND
				}
			}

		case POLICY_CATEGORY_NAME:
			if len(first.Name) != len(second.Name) {
				if item.value < 0 && len(first.Name) < len(second.Name) {
					return DELETE_WHICH_FIRST
				} else {
					return DELETE_WHICH_SECOND
				}
			}

		case POLICY_CATEGORY_PATH:
			if len(first.Path) != len(second.Path) {
				if item.value < 0 && len(first.Path) < len(second.Path) {
					return DELETE_WHICH_FIRST
				} else {
					return DELETE_WHICH_SECOND
				}
			}
		}
	}

	// No rule for the two files, then we could delete either.
	return DELETE_WHICH_EITHER
}

// Get policy item based on name.
func getPolicyItem(name string) *policyItem {
	if item, found := policyItemMapping[name]; found {
		return item
	} else {
		return nil
	}
}

// Check if a policy item exists in an array.
func policyItemExist(items []*policyItem, category int) bool {
	for _, item := range items {
		if item.category == category {
			return true
		}
	}

	return false
}

// Create a new policy object.
func NewPolicy(spec string) (Policy, error) {

	// Number of items is always fixed.
	items := make([]*policyItem, POLICY_CATEGORY_COUNT)
	var count int = 0

	// Parse user spec.
	if len(spec) > 0 {
		for _, name := range strings.Split(strings.ToLower(spec), ",") {
			if newItem := getPolicyItem(name); newItem == nil {
				return nil, E_INVALID_POLICY
			} else {
				// Check if the new item is duplicated.
				if count > 0 && policyItemExist(items[0:count], newItem.category) {
					return nil, E_INVALID_POLICY
				}

				// Add the new item to the array.
				items[count] = newItem
				count++
			}
		}
	}

	// If any item is missing in user spec,
	// then add it to the end.
	if count < POLICY_CATEGORY_COUNT {
		for _, value := range defaultPolicyItems {
			if !policyItemExist(items[0:count], value.category) {
				// Add the new item to the array.
				items[count] = value
				count++

				if count == POLICY_CATEGORY_COUNT {
					break
				}
			}
		}
	}

	return &policyImpl{items}, nil
}
