// File deduplication
package main

import (
	"os"
	"path/filepath"
	"strings"
)

func SamePath(path1, path2 string) bool {
	if os.PathSeparator == '/' {
		return path1 == path2
	} else {
		return strings.EqualFold(path1, path2)
	}
}

func SameOrInFolder(parent, child string) bool {
	// Parent path should not be longer than child.
	if len(parent) > len(child) {
		return false
	}

	// If they have the same length, the do a simple comparison.
	if len(parent) == len(child) {
		return SamePath(parent, child)
	}

	// Child path is longer than parent,
	// check if there is a path separator at the right position.
	if child[len(parent)] != os.PathSeparator {
		return false
	}

	if os.PathSeparator == '/' {
		// Case sensitive check.
		return strings.HasPrefix(child, parent)
	} else {
		// Case insensitive check (Windows).
		return strings.EqualFold(parent, child[0:len(parent)])
	}
}

func GetPathAsKey(path string) string {
	if os.PathSeparator == '/' {
		return path
	} else {
		return strings.ToLower(path)
	}
}

func GetBaseName(path string) (string, bool) {
	index := strings.LastIndexByte(path, os.PathSeparator)

	if index == -1 || index == (len(path)-1) {
		return "", false
	}

	return path[index+1:], true
}

// Generate a new path.
//
// For instance,
// 1) folder: "/aa/bb/cc"
// 2) name: "dd"
// 3) return value will be: "/aa/bb/cc/dd"
func AppendPath(folder string, name string) string {
	if len(folder) > 0 && folder[len(folder)-1] == os.PathSeparator {
		return folder + name
	} else {
		return folder + string(os.PathSeparator) + name
	}
}

// Get absolute path.
//
// 1) There is no path separator at the end of returned string.
// 2) If input string is "/", then "" would be returned.
func GetAbsPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	abs = filepath.Clean(abs)
	return strings.TrimRight(abs, string(os.PathSeparator)), nil
}
