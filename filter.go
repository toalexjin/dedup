// File deduplication
package main

import (
	"os/user"
	"path/filepath"
	"sort"
	"strings"
)

// Filter interface.
type Filter interface {
	// Get cache directory ($HOME/.dedup).
	GetCacheDir() string

	// Get filter specification.
	GetSpec() string

	// Check if a folder or file need to skip.
	Skip(path string, isDir bool) bool
}

type filterImpl struct {
	// "$HOME/.dedup"
	cacheDir string

	// Include Extentions.
	includeExts []string

	// Filter specification.
	spec string
}

var extentionMap = map[string][]string{
	"photo": {".bmp", ".gif", ".jpeg", ".jpg", ".png", ".tiff"},
	"video": {".avi", ".mp4", ".mpg", ".rm", ".wmv"},
}

func parseTypes(types string) ([]string, error) {
	result := make([]string, 0, 15)

	for _, value := range strings.Split(strings.ToLower(types), ",") {
		if list, ok := extentionMap[value]; ok {
			for _, str := range list {
				result = append(result, str)
			}
		} else {
			return nil, ErrInvalidFileTypes
		}
	}

	// Sort it in ascending order.
	sort.Strings(result)

	return result, nil
}

// Create a new filter object.
func NewFilter(types string) (Filter, error) {
	filter := new(filterImpl)

	// Get $HOME.
	if current, err := user.Current(); err != nil {
		panic(err)
	} else {
		// Get "$HOME/.dedup".
		filter.cacheDir = filepath.Join(current.HomeDir, ".dedup")
	}

	// Parse types.
	if len(types) > 0 {
		var err error
		if filter.includeExts, err = parseTypes(types); err != nil {
			return nil, err
		}
	}

	// Generate spec.
	for _, ext := range filter.includeExts {
		if len(filter.spec) > 0 {
			filter.spec += ","
		}

		filter.spec += ext
	}

	return filter, nil
}

func (me *filterImpl) GetCacheDir() string {
	return me.cacheDir
}

// Get filter specification.
func (me *filterImpl) GetSpec() string {
	return me.spec
}

func (me *filterImpl) Skip(path string, isDir bool) bool {
	// If it's in cache folder, then skip it.
	if SameOrIsChild(me.cacheDir, path) {
		return true
	}

	// If it's a folder or include extention list
	// is empty, then do not skip it.
	if isDir || len(me.includeExts) == 0 {
		return false
	}

	// If file extention is in the include list,
	// then do not skip it.
	ext := strings.ToLower(filepath.Ext(path))

	// Find the extention in the sorted list.
	index := sort.SearchStrings(me.includeExts, ext)
	if index < len(me.includeExts) && me.includeExts[index] == ext {
		return false
	}

	// Otherwise, skip it.
	return true
}
