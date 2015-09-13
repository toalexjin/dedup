// File deduplication
package main

import (
	"errors"
)

var (
	ErrInvalidPolicy = errors.New("Invalid command line policy argument.")
	ErrInvalidCacheFile = errors.New("Invalid cache file content.")
)
