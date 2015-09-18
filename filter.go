// File deduplication
package main

import (
	"os/user"
	"path/filepath"
	"strings"
)

// Filter interface.
type Filter interface {
	// Get cache directory ($HOME/.dedup).
	GetCacheDir() string

	// Check if a folder or file needs to skip.
	Skip(path, name string, isDir bool) bool
}

type filterImpl struct {
	// "$HOME/.dedup"
	cacheDir string

	// Include Extentions.
	includeExts map[string]bool

	// Exclude Extentions.
	excludeExts map[string]bool
}

var extentionMapping = map[string][]string{
	"audio":   {".mp3"},
	"office":  {".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx"},
	"photo":   {".bmp", ".gif", ".jpeg", ".jpg", ".png", ".tiff"},
	"video":   {".avi", ".mp4", ".mpg", ".rm", ".wmv"},
	"tarball": {".7z", ".bz", ".gz", ".iso", ".rar", ".tar.gz", ".tgz", ".zip"},
}

func parseTypes(exts map[string]bool, types string) error {
	for _, value := range strings.Split(strings.ToLower(types), ",") {
		list, ok := extentionMapping[value]

		// If it could not be found, then return error.
		if !ok {
			return ErrInvalidFilters
		}

		// Add all extentions to map.
		for _, str := range list {
			exts[str] = true
		}
	}

	return nil
}

// Create a new filter object.
func NewFilter(includes, excludes string) (Filter, error) {
	filter := &filterImpl{
		includeExts: make(map[string]bool),
		excludeExts: make(map[string]bool),
	}

	// Get $HOME.
	if current, err := user.Current(); err != nil {
		panic(err)
	} else {
		// Get "$HOME/.dedup".
		filter.cacheDir = filepath.Join(current.HomeDir, ".dedup")
	}

	// Include filters.
	if len(includes) > 0 {
		if err := parseTypes(filter.includeExts, includes); err != nil {
			return nil, err
		}
	}

	// Exclude filters.
	if len(excludes) > 0 {
		if err := parseTypes(filter.excludeExts, excludes); err != nil {
			return nil, err
		}
	}

	return filter, nil
}

func (me *filterImpl) GetCacheDir() string {
	return me.cacheDir
}

func (me *filterImpl) Skip(path, name string, isDir bool) bool {
	// If it's in cache folder, then skip it.
	if SameOrIsChild(me.cacheDir, path) {
		return true
	}

	// Include and exclude filters are for files only,
	// NOT for folders.
	if isDir {
		return false
	}

	// If both include and exclude filter are empty,
	// then do NOT skip this path.
	if len(me.includeExts) == 0 && len(me.excludeExts) == 0 {
		return false
	}

	// Get file extention.
	ext := strings.ToLower(filepath.Ext(name))

	// If it's included by exclude filters, then skip it.
	if _, ok := me.excludeExts[ext]; ok {
		return true
	}

	// Include filters.
	if len(me.includeExts) > 0 {
		if _, ok := me.includeExts[ext]; ok {
			return false
		}

		// If include filters have been set, then any files
		// that are not included by include filters will be skipped.
		return true
	}

	return false
}
