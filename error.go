// File deduplication
package main

import (
	"errors"
)

var (
	ErrInvalidPolicy        = errors.New("Invalid policy argument (-p <POLICY>,...).")
	ErrInvalidCacheFile     = errors.New("Invalid cache file format.")
	ErrRootPathNotPermitted = errors.New("Root path (\"/\") is not permitted.")
	ErrInvalidFileTypes     = errors.New("Invalid file type argument (-t <TYPE>,...).")
)
