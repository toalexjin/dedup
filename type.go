// File deduplication
package main

// File attributes.
type FileAttr struct {
	Path       string   // Full path.
	ModTime    int64    // the number of nanoseconds elapsed since January 1, 1970 UTC
	Size       int64    // File size, in bytes.
	SHA256     [32]byte // SHA256 checksum.
	StillExist bool     // Indicates if the file still exists.
}

// Update status.
type Updater interface {
	Error() error                          // Any error happened or job was cancelled.
	SetError(err error)                    // Set error code.
	Print(format string, a ...interface{}) // Print status message.
}
