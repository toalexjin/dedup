// File deduplication
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"strings"
)

// SHA256 hash value
type SHA256Digest [sha256.Size]byte

// Convert sha256 to a string.
func (me *SHA256Digest) String() string {
	return hex.EncodeToString((*me)[:])
}

// File attributes.
type FileAttr struct {
	Path       string       // Full path.
	Name       string       // Name.
	ModTime    int64        // the number of nanoseconds elapsed since January 1, 1970 UTC
	Size       int64        // File size, in bytes.
	SHA256     SHA256Digest // SHA256 checksum.
	StillExist bool         // Indicates if the file still exists.
}

// File scanner interface.
type FileScanner interface {

	// Get path.
	GetPath() string

	// Get total files.
	//
	// This function should be called after scanning files.
	GetTotalFiles() int

	// Get total folders.
	//
	// This function should be called after scanning files.
	GetTotalFolders() int

	// Get total size, in bytes.
	//
	// This function should be called after scanning files.
	GetTotalBytes() int64

	// Get all files
	//
	// The map key is file full path,
	// which might be lower case on Windows platform.
	GetFiles() map[string]*FileAttr

	// Remove a file from the map.
	//
	// This function is not used to remove file from disk.
	Remove(path string)

	// Scan files
	Scan(updater Updater) error

	// Save file hashes to speed up next scan.
	Save() error
}

// File scanner implementation.
type fileScannerImpl struct {
	path         string               // File path.
	info         os.FileInfo          // File information.
	files        map[string]*FileAttr // All files.
	totalFiles   int                  // Total files.
	totalFolders int                  // Total folders.
	totalBytes   int64                // Total size, in bytes.
	hashEngine   hash.Hash            // SHA256 hash engine.
	buffer       []byte               // Buffer for reading file content.
	dirty        bool                 // Indicates if any file has been removed.
}

// Create a new file scanner.
func NewFileScanner(path string, info os.FileInfo) FileScanner {
	return &fileScannerImpl{
		path: path, info: info,
		files:      make(map[string]*FileAttr),
		hashEngine: sha256.New(),
		buffer:     make([]byte, 16*1024),
	}
}

func (me *fileScannerImpl) GetPath() string {
	return me.path
}

func (me *fileScannerImpl) GetTotalFiles() int {
	return me.totalFiles
}

func (me *fileScannerImpl) GetTotalFolders() int {
	return me.totalFolders
}

func (me *fileScannerImpl) GetTotalBytes() int64 {
	return me.totalBytes
}

func (me *fileScannerImpl) GetFiles() map[string]*FileAttr {
	return me.files
}

func (me *fileScannerImpl) Remove(path string) {
	// File path is the key.
	var key string = path

	// Convert file path to lower case on Windows.
	if os.PathSeparator != '/' {
		key = strings.ToLower(key)
	}

	delete(me.files, key)
	me.dirty = true
}

func (me *fileScannerImpl) Scan(updater Updater) error {
	// Load file list of the previous scan.
	me.loadCache()

	// Scan files.
	if me.info.IsDir() {
		if err := me.scanFolder(updater); err != nil {
			return err
		}
	} else {
		if err := me.scanFile(me.path, me.info, updater); err != nil {
			return err
		}
	}

	// Some files do not exist in disk any more,
	// let's remove them from the map.
	me.removeNonExistFiles()

	return nil
}

// Scan folder recursively.
func (me *fileScannerImpl) scanFolder(updater Updater) error {

	var head, tail int = 0, 1
	folders := make([]string, 0, 64)
	folders = append(folders, me.path)

	for head < tail {
		// Check if job was cancelled or an error ever happened.
		if err := updater.Error(); err != nil {
			return err
		}

		// Pop a folder path.
		path := folders[head]
		head++

		updater.Log(LOG_TRACE, "Scanning folder %v...", path)

		// Open this folder.
		fp, err := os.Open(path)
		if err != nil {
			updater.Log(LOG_ERROR, "Could not open folder %v. Error:%v", path, err)
			updater.SetError(err)
			return err
		}

		for {
			items, errReadDir := fp.Readdir(512)
			if errReadDir != nil && errReadDir != io.EOF {
				updater.Log(LOG_ERROR, "Could not enumerate folder %v. Error:%v", path, errReadDir)
				updater.SetError(errReadDir)

				// Close the folder
				fp.Close()
				return errReadDir
			}

			for i := 0; i < len(items); i++ {
				// Check if job was cancelled or an error ever happened.
				if err := updater.Error(); err != nil {
					// Close the folder
					fp.Close()
					return err
				}

				var subPath string

				if len(path) == 1 && path == "/" {
					subPath = path + items[i].Name()
				} else {
					subPath = path + string(os.PathSeparator) + items[i].Name()
				}

				if items[i].IsDir() {
					// Push the sub-folder path to the end.
					folders = append(folders, subPath)
					tail++
					me.totalFolders++
				} else if items[i].Mode().IsRegular() {
					if err := me.scanFile(subPath, items[i], updater); err != nil {
						// Close the folder
						fp.Close()
						return err
					}
				}
			}

			// If reaching end of the folder, then break.
			if errReadDir == io.EOF {
				break
			}
		}

		// Close the folder
		fp.Close()
	}

	return nil
}

// Calculate file checksum and put it to the map.
func (me *fileScannerImpl) scanFile(
	path string, info os.FileInfo, updater Updater) error {

	// File path is map key.
	var key string = path

	// Convert file path to lower case on Windows.
	if os.PathSeparator != '/' {
		key = strings.ToLower(key)
	}

	// If the file already exists in the map,
	// and file size & last modification time are the same,
	// then skip to read file content to enhance performance.
	if value, found := me.files[key]; found {
		if value.Size == info.Size() && value.ModTime == info.ModTime().UnixNano() {
			// Write trace log message.
			updater.Log(LOG_TRACE, "%v (%v)", path, &value.SHA256)

			// Update file count and total size.
			me.totalFiles++
			me.totalBytes += info.Size()

			value.StillExist = true
			return nil
		}
	}

	// Open file.
	fp, err := os.Open(path)
	if err != nil {
		updater.Log(LOG_ERROR, "Could not open file %v. Error:%v", path, err)
		return err
	}
	defer fp.Close()

	// Reset hash engine
	me.hashEngine.Reset()

	// Read file content
	for {
		// Check if job was cancelled or an error ever happened.
		if err := updater.Error(); err != nil {
			return err
		}

		n, err := fp.Read(me.buffer)
		if err != nil && err != io.EOF {
			updater.Log(LOG_ERROR, "Could not read file %v. Error:%v", path, err)
			return err
		}
		me.hashEngine.Write(me.buffer[0:n])

		if err == io.EOF {
			break
		}
	}

	// Create a new object.
	newValue := &FileAttr{
		Path:       path,
		Name:       info.Name(),
		ModTime:    info.ModTime().UnixNano(),
		Size:       info.Size(),
		StillExist: true,
	}
	copy(newValue.SHA256[:], me.hashEngine.Sum(nil))

	// Add the new object to map.
	me.files[key] = newValue

	// Update file count and total size.
	me.totalFiles++
	me.totalBytes += info.Size()

	// Write a trace log message.
	updater.Log(LOG_TRACE, "%v (%v)", path, &newValue.SHA256)

	return nil
}

// Some files do not exist in disk any more,
// let's remove them from the map.
func (me *fileScannerImpl) removeNonExistFiles() {
	for key, value := range me.files {
		if !value.StillExist {
			delete(me.files, key)
		}
	}
}

func (me *fileScannerImpl) loadCache() {
	// TO-DO: Load cache from local disk.
}

func (me *fileScannerImpl) Save() error {
	if !me.dirty {
		return nil
	}

	// Reset dirty flag
	me.dirty = false

	// TO-DO: Save file hashes to disk
	//        in order to speed up next scan.
	return nil
}
