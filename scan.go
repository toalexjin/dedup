// File deduplication
package main

import (
	"crypto/sha256"
	"hash"
	"io"
	"os"
	"strings"
)

// File attributes.
type FileAttr struct {
	Path       string   // Full path.
	Name       string   // Name.
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

// Scan a path (could be file or folder).
func ScanPath(path string, files map[string]*FileAttr,
	updater Updater) error {

	// Check if it's file or folder.
	info, err := os.Stat(path)
	if err != nil {
		updater.Print("Path %v might not exist. Error:%v", path, err)
		updater.SetError(err)
		return err
	}

	// Create hash engine and allocate buffer to read file.
	hash := sha256.New()
	buffer := make([]byte, 16*1024)

	if info.IsDir() {
		if err = scanFolder_i(path, files,
			updater, hash, buffer); err != nil {
			return err
		}
	} else {
		if err = scanFile_i(path, files,
			updater, hash, buffer, info); err != nil {
			return err
		}
	}

	// Some files do not exist in disk any more,
	// let's remove them from the map.
	removeNonExistFiles(files)

	return nil
}

// Scan a folder and its sub-folders recursively.
func scanFolder_i(path string, files map[string]*FileAttr,
	updater Updater, hash hash.Hash, buffer []byte) error {
	fp, err := os.Open(path)
	if err != nil {
		updater.Print("Could not open folder %v. Error:%v", path, err)
		updater.SetError(err)
		return err
	}
	defer fp.Close()

	for {
		list, err := fp.Readdir(256)
		if err != nil && err != io.EOF {
			updater.Print("Could not enumerate folder %v. Error:%v", path, err)
			updater.SetError(err)
			return err
		}

		for i := 0; i < len(list); i++ {
			// Check if job was cancelled or an error ever happened.
			if err := updater.Error(); err != nil {
				return err
			}

			// Get full path.
			var fullPath string

			if len(path) == 1 && path == "/" {
				fullPath = path + list[i].Name()
			} else {
				fullPath = path + string(os.PathSeparator) + list[i].Name()
			}

			if list[i].IsDir() {
				if tmp := scanFolder_i(fullPath, files,
					updater, hash, buffer); tmp != nil {
					return tmp
				}
			} else {
				if tmp := scanFile_i(fullPath, files,
					updater, hash, buffer, list[i]); tmp != nil {
					return tmp
				}
			}
		}

		// If reaching end of the folder, then break.
		if err == io.EOF {
			break
		}
	}

	return nil
}

// Calculate single file checksum.
func scanFile_i(path string, files map[string]*FileAttr,
	updater Updater, hash hash.Hash, buffer []byte,
	info os.FileInfo) error {

	var key string = path

	// Case insensitive on Windows.
	if os.PathSeparator != '/' {
		key = strings.ToLower(key)
	}

	// If the file already exists in the map,
	// and file size & last modification time are the same,
	// then skip to read it to enhance performance.
	if value, found := files[key]; found {
		if value.Size == info.Size() && value.ModTime == info.ModTime().UnixNano() {
			value.StillExist = true
			return nil
		}
	}

	// Open file.
	fp, err := os.Open(path)
	if err != nil {
		updater.Print("Could not open file %v. Error:%v", path, err)
		return err
	}
	defer fp.Close()

	// Reset hash engine
	hash.Reset()

	// Read file content
	for {
		// Check if job was cancelled or an error ever happened.
		if err := updater.Error(); err != nil {
			return err
		}

		n, err := fp.Read(buffer)
		if err != nil && err != io.EOF {
			updater.Print("Could not read file %v. Error:%v", path, err)
			return err
		}
		hash.Write(buffer[0:n])

		if err == io.EOF {
			break
		}
	}

	// Create a new object.
	newValue := &FileAttr{
		Path:       path,
		ModTime:    info.ModTime().UnixNano(),
		Size:       info.Size(),
		StillExist: true,
	}
	copy(newValue.SHA256[:], hash.Sum(nil))

	// Add the new object to map.
	files[key] = newValue

	return nil
}

// Some files do not exist in disk any more, let's remove them from the map.
func removeNonExistFiles(files map[string]*FileAttr) {
	for key, value := range files {
		if !value.StillExist {
			delete(files, key)
		}
	}
}
