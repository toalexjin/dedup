// File deduplication
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
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
	Path    string       // Full path.
	Name    string       // Name.
	ModTime int64        // the number of nanoseconds elapsed since January 1, 1970 UTC
	Size    int64        // File size, in bytes.
	SHA256  SHA256Digest // SHA256 checksum.

	// Detailed information.
	//
	// With detailed information, we could know if two files
	// are the same by calling os.SameFile().
	//
	// After loading saved file list from local disk cache,
	// this field is null. While scanning files,
	// this field will be set to valid value.
	// If thie field is still null after scanning files,
	// then it means that the file no longer exists in disk
	// and should be removed from map.
	Details os.FileInfo
}

func (me *FileAttr) String() string {
	return fmt.Sprintf("%v(%v,%v bytes,%v)",
		me.Path, me.Name, me.Size, &me.SHA256)
}

// Read a FileAttr object from cache file.
func (me *FileAttr) ReadCache(reader *bufio.Reader) error {
	var str string

	for {
		line, isPrefix, err := reader.ReadLine()
		if err != nil {
			return err
		}

		if len(str) == 0 {
			str = string(line)
		} else {
			str += string(line)
		}

		if !isPrefix {
			break
		}
	}

	fields := strings.Split(str, "|")
	if len(fields) != 4 {
		return ErrInvalidCacheFile
	}

	if !filepath.IsAbs(fields[0]) {
		return ErrInvalidCacheFile
	}

	// Path.
	me.Path = fields[0]

	// Name.
	index := strings.LastIndexByte(me.Path, os.PathSeparator)
	if index == -1 || index == (len(me.Path)-1) {
		return ErrInvalidCacheFile
	}
	pathBytes := []byte(me.Path)
	me.Name = string(pathBytes[index+1:])

	// Mod time.
	if number, err := strconv.ParseInt(fields[1], 10, 64); err != nil {
		return err
	} else if number < 0 {
		return ErrInvalidCacheFile
	} else {
		me.ModTime = number
	}

	// Size.
	if number, err := strconv.ParseInt(fields[2], 10, 64); err != nil {
		return err
	} else if number < 0 {
		return ErrInvalidCacheFile
	} else {
		me.Size = number
	}

	// SHA256 Hash.
	if digest, err := hex.DecodeString(fields[3]); err != nil {
		return err
	} else if len(digest) != sha256.Size {
		return ErrInvalidCacheFile
	} else {
		copy(me.SHA256[:], digest)
	}

	// Field "Details" now is null, will be set to
	// valid value when scanning files.
	me.Details = nil

	return nil
}

// Write a FileAttr object to cache file.
func (me *FileAttr) SaveCache(writer *bufio.Writer) error {
	str := fmt.Sprintf("%v|%v|%v|%v\n",
		me.Path, me.ModTime, me.Size, &me.SHA256)

	_, err := writer.WriteString(str)
	return err
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
	Scan() error

	// Read cache saved by previous scan.
	ReadCache() error

	// Save file hashes to speed up next scan.
	SaveCache() error
}

// File scanner implementation.
type fileScannerImpl struct {
	path         string               // File path.
	info         os.FileInfo          // File information.
	updater      Updater              // Updater interface
	files        map[string]*FileAttr // All files.
	totalFiles   int                  // Total files.
	totalFolders int                  // Total folders.
	totalBytes   int64                // Total size, in bytes.
	hashEngine   hash.Hash            // SHA256 hash engine.
	buffer       []byte               // Buffer for reading file content.
	dirty        bool                 // Indicates if any file has been removed.
}

// Create a new file scanner.
func NewFileScanner(path string,
	info os.FileInfo, updater Updater) FileScanner {
	return &fileScannerImpl{
		path:       path,
		info:       info,
		updater:    updater,
		files:      make(map[string]*FileAttr),
		hashEngine: sha256.New(),
		buffer:     make([]byte, 512*1024),
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

	// A file was removed from the map,
	// set dirty flag to true.
	me.dirty = true
}

func (me *fileScannerImpl) Scan() error {
	// Scan files.
	if me.info.IsDir() {
		if err := me.scanFolder(); err != nil {
			return err
		}
	} else {
		me.scanFile(me.path, me.info)
	}

	// Some files do not exist in disk any more,
	// let's remove them from the map.
	me.removeNonExistFiles()

	return nil
}

// Scan folder recursively.
func (me *fileScannerImpl) scanFolder() error {

	var head, tail int = 0, 1
	folders := make([]string, 0, 64)
	folders = append(folders, me.path)

	for head < tail {
		// Check if fatal error ever happened.
		if err := me.updater.FatalError(); err != nil {
			return err
		}

		// Pop a folder path.
		path := folders[head]
		head++

		if len(path) > len(me.path) {
			me.updater.Log(LOG_TRACE, "Scanning %v...", path)
		}

		// Open this folder.
		fp, err := os.Open(path)
		if err != nil {
			me.updater.IncreaseErrors()
			me.updater.Log(LOG_ERROR, "Could not open folder %v. Error:%v", path, err)
			continue
		}

		for {
			items, errReadDir := fp.Readdir(512)
			if errReadDir != nil && errReadDir != io.EOF {
				me.updater.IncreaseErrors()
				me.updater.Log(LOG_ERROR, "Could not enumerate folder %v. Error:%v", path, errReadDir)
				break
			}

			for i := 0; i < len(items); i++ {
				// Check if fatal error ever happened.
				if err := me.updater.FatalError(); err != nil {
					// Close the folder
					fp.Close()
					return err
				}

				var subPath string

				if len(path) > 0 && path[len(path)-1] == os.PathSeparator {
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
					me.scanFile(subPath, items[i])
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
	path string, info os.FileInfo) error {

	// File path is map key.
	key := path

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
			me.updater.Log(LOG_TRACE, "%v (%v)", path, &value.SHA256)

			// Update file count and total size.
			me.totalFiles++
			me.totalBytes += info.Size()

			// Set FileAttr.Details to valid value.
			value.Details = info
			return nil
		}
	}

	// Open file.
	fp, err := os.Open(path)
	if err != nil {
		me.updater.IncreaseErrors()
		me.updater.Log(LOG_ERROR, "Could not open file %v. Error:%v", path, err)
		return err
	}
	defer fp.Close()

	// Reset hash engine
	me.hashEngine.Reset()

	// Read file content
	for {
		// Check if fatal error ever happened.
		if err := me.updater.FatalError(); err != nil {
			return err
		}

		n, err := fp.Read(me.buffer)
		if err != nil && err != io.EOF {
			me.updater.IncreaseErrors()
			me.updater.Log(LOG_ERROR, "Could not read file %v. Error:%v", path, err)
			return err
		}
		me.hashEngine.Write(me.buffer[0:n])

		if err == io.EOF {
			break
		}
	}

	// Create a new object.
	newValue := &FileAttr{
		Path:    path,
		Name:    info.Name(),
		ModTime: info.ModTime().UnixNano(),
		Size:    info.Size(),
		Details: info,
	}
	copy(newValue.SHA256[:], me.hashEngine.Sum(nil))

	// Add the new object to map.
	me.files[key] = newValue

	// Update file count and total size.
	me.totalFiles++
	me.totalBytes += info.Size()

	// A new file was added to the map,
	// set dirty flag to true.
	me.dirty = true

	// Write a trace log message.
	me.updater.Log(LOG_TRACE, "%v (%v)", path, &newValue.SHA256)

	return nil
}

// Some files do not exist in disk any more,
// let's remove them from the map.
func (me *fileScannerImpl) removeNonExistFiles() {
	for key, value := range me.files {
		if value.Details == nil {
			delete(me.files, key)
		}
	}
}

func (me *fileScannerImpl) getCacheFile(readonly bool) (string, error) {

	// Get user home directory.
	current, err := user.Current()
	if err != nil {
		return "", err
	}

	// Get the directory where cache files are located.
	dir := filepath.Join(current.HomeDir, ".dedup")

	// Create the directory if it does not exist.
	if !readonly {
		if _, err := os.Stat(dir); err != nil {
			if err := os.Mkdir(dir, os.ModePerm); err != nil {
				return "", err
			}
		}
	}

	key := me.path
	if os.PathSeparator != '/' {
		key = strings.ToLower(key)
	}

	me.hashEngine.Reset()
	me.hashEngine.Write([]byte(key))
	name := hex.EncodeToString(me.hashEngine.Sum(nil)) + ".dat"

	path := dir + string(os.PathSeparator) + name

	return path, nil
}

func (me *fileScannerImpl) ReadCache() error {
	// Get cache file path.
	cache, err := me.getCacheFile(true)
	if err != nil {
		return err
	}

	// Print trace log message.
	me.updater.Log(LOG_TRACE, "Reading cache %v...", cache)

	// Open cache file.
	fp, err := os.Open(cache)
	if err != nil {
		if err == os.ErrNotExist {
			return nil
		} else {
			return err
		}
	}
	defer fp.Close()

	// Create a buffered reader to enhance read performance.
	reader := bufio.NewReader(fp)

	for {
		object := new(FileAttr)
		err := object.ReadCache(reader)

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		me.updater.Log(LOG_TRACE, "Cache info: %v", object)

		// File path is map key.
		key := object.Path

		// Convert file path to lower case on Windows.
		if os.PathSeparator != '/' {
			key = strings.ToLower(key)
		}

		me.files[key] = object
	}

	return nil
}

func (me *fileScannerImpl) SaveCache() error {

	if !me.dirty {
		return nil
	}

	me.dirty = false

	// Get cache file path.
	cache, err := me.getCacheFile(false)
	if err != nil {
		return err
	}

	// Print trace log message.
	me.updater.Log(LOG_TRACE, "Updating cache %v...", cache)

	// Create a new cache file.
	fp, err := os.OpenFile(cache, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer fp.Close()

	// Create a buffered writer to enhance performance.
	writer := bufio.NewWriter(fp)

	// Write all files with their hashes to disk.
	for _, object := range me.files {
		if err := object.SaveCache(writer); err != nil {
			return err
		}
	}

	// If it's a buffered writer, we need to flush data to disk.
	if err := writer.Flush(); err != nil {
		return err
	}

	return nil
}
