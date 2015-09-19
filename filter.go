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
	"audio": {".aac", ".ac3", ".amr", ".ape", ".cda",
		".dts", ".flac", ".m1a", ".m2a", ".m4a",
		".mka", ".mp2", ".mp3", ".mpa", ".ra",
		".tta", ".wav", ".wma", ".wv", ".mid"},

	"office": {".doc", ".dot", ".docx", ".docm", ".dotx", ".dotm", ".docb",
		".xls", ".xlt", ".xlm", ".xlsx", ".xlsm", ".xltx", ".xltm",
		".ppt", ".pot", ".pps", ".pptx", ".pptm", ".potx", ".potm",
		".ppam", ".ppsx", ".ppsm", ".sldx", ".sldm",
		".mdb", ".accdb", ".accde", ".accdt", ".accdr"},

	"photo": {".bmp", ".emf", ".gif", ".ico", ".jpeg",
		".jpg", ".png", ".psd", ".svg", ".tiff", ".wmf"},

	"video": {".asf", ".avi", ".mov", ".mp4", ".mpg",
		".rm", ".rmvb", ".vob", ".wmv"},

	"tarball": {".7z", ".ace", ".ar", ".arc", ".ari",
		".arj", ".bz", ".bz2", ".bzip2", ".cab",
		".gho", ".gz", ".gzi", ".gzip", ".iso",
		".rar", ".tar", ".tar.gz", ".tgz", ".z",
		".zip", ".zipx", ".zz"},
}

func SupportView(ext string) bool {
	for key, list := range extentionMapping {
		if key != "audio" && key != "office" && key != "photo" && key != "video" {
			continue
		}

		for _, value := range list {
			if strings.EqualFold(value, ext) {
				return true
			}
		}
	}

	return false
}

func parseTypes(types string, exts map[string]bool) error {
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
		if err := parseTypes(includes, filter.includeExts); err != nil {
			return nil, err
		}
	}

	// Exclude filters.
	if len(excludes) > 0 {
		if err := parseTypes(excludes, filter.excludeExts); err != nil {
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
