// File deduplication
package main

import (
	"os/user"
	"path/filepath"
)

// Filter interface.
type Filter interface {
	// Get cache directory ($HOME/.dedup).
	GetCacheDir() string

	// Check if a folder or file need to skip.
	Skip(path string) bool
}

type filterImpl struct {
	// "$HOME/.dedup"
	cacheDir string
}

// Create a new filter object.
func NewFilter() Filter {
	filter := new(filterImpl)

	// Get $HOME.
	if current, err := user.Current(); err != nil {
		panic(err)
	} else {
		// Get "$HOME/.dedup".
		filter.cacheDir = filepath.Join(current.HomeDir, ".dedup")
	}

	return filter
}

func (me *filterImpl) GetCacheDir() string {
	return me.cacheDir
}

func (me *filterImpl) Skip(path string) bool {
	return SameOrIsChild(me.cacheDir, path)
}
