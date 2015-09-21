// File deduplication
package main

import (
	"os"
	"strings"
)

const (
	// The first file should be removed.
	DELETE_WHICH_FIRST = iota

	// The second file should be removed.
	DELETE_WHICH_SECOND

	// Either could be removed.
	DELETE_WHICH_EITHER

	// Neither could be removed.
	DELETE_WHICH_NEITHER
)

const (
	// -1 means old and 1 means new.
	POLICY_CATEGORY_MOD_TIME = iota

	// -1 means short name and 1 means long name.
	POLICY_CATEGORY_NAME

	// -1 means short path and 1 means long path.
	POLICY_CATEGORY_PATH
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
	&policyItem{category: POLICY_CATEGORY_NAME, value: 1},
	&policyItem{category: POLICY_CATEGORY_PATH, value: 1},
	&policyItem{category: POLICY_CATEGORY_MOD_TIME, value: 1},
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
	// We check file size here to avoid hash collision.
	if first.Size != second.Size {
		return DELETE_WHICH_NEITHER
	}

	// Check if the two paths are identical.
	if SamePath(first.Path, second.Path) {
		return DELETE_WHICH_NEITHER
	}

	// If a folder is symbolic link, then different
	// file paths might point to the same file.
	// To avoid deleting file by mistake,
	// we have to call os.SameFile().
	if os.SameFile(first.Details, second.Details) {
		return DELETE_WHICH_NEITHER
	}

	for _, item := range me.items {
		switch item.category {
		case POLICY_CATEGORY_MOD_TIME:
			if first.ModTime != second.ModTime {
				if first.ModTime < second.ModTime {
					if item.value < 0 {
						return DELETE_WHICH_FIRST
					} else {
						return DELETE_WHICH_SECOND
					}
				} else {
					if item.value < 0 {
						return DELETE_WHICH_SECOND
					} else {
						return DELETE_WHICH_FIRST
					}
				}
			}

		case POLICY_CATEGORY_NAME:
			if len(first.Name) != len(second.Name) {
				if len(first.Name) < len(second.Name) {
					if item.value < 0 {
						return DELETE_WHICH_FIRST
					} else {
						return DELETE_WHICH_SECOND
					}
				} else {
					if item.value < 0 {
						return DELETE_WHICH_SECOND
					} else {
						return DELETE_WHICH_FIRST
					}
				}
			}

		case POLICY_CATEGORY_PATH:
			if len(first.Path) != len(second.Path) {
				if len(first.Path) < len(second.Path) {
					if item.value < 0 {
						return DELETE_WHICH_FIRST
					} else {
						return DELETE_WHICH_SECOND
					}
				} else {
					if item.value < 0 {
						return DELETE_WHICH_SECOND
					} else {
						return DELETE_WHICH_FIRST
					}
				}
			}
		}
	}

	// No rule for the two files, then we could delete either.
	return DELETE_WHICH_EITHER
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
			if newItem, ok := policyItemMapping[name]; ok {
				// Check if the new item is duplicated.
				if count > 0 && policyItemExist(items[0:count], newItem.category) {
					return nil, ErrInvalidPolicy
				}

				// Add the new item to the array.
				items[count] = newItem
				count++
			} else {
				return nil, ErrInvalidPolicy
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
