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
	ModTime int64        // The number of nanoseconds elapsed since January 1, 1970 UTC
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

	// Start to parse the line.
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
	if name, ok := GetBaseName(me.Path); ok {
		me.Name = name
	} else {
		return ErrInvalidCacheFile
	}

	// Mod time.
	if number, err := strconv.ParseInt(fields[1], 10, 64); err != nil {
		return ErrInvalidCacheFile
	} else if number < 0 {
		return ErrInvalidCacheFile
	} else {
		me.ModTime = number
	}

	// Size.
	if number, err := strconv.ParseInt(fields[2], 10, 64); err != nil {
		return ErrInvalidCacheFile
	} else if number < 0 {
		return ErrInvalidCacheFile
	} else {
		me.Size = number
	}

	// SHA256 Hash.
	if digest, err := hex.DecodeString(fields[3]); err != nil {
		return ErrInvalidCacheFile
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

	// Get scanned files.
	GetScannedFiles() map[SHA256Digest][]*FileAttr

	// File removed event.
	//
	// This event is used to update cache file.
	OnFileRemoved(removed *FileAttr)

	// Scan files
	Scan() error

	// Read cache saved by previous scan.
	ReadCache() error

	// Save file hashes to speed up next scan.
	SaveCache() error
}

// File scanner implementation.
type fileScannerImpl struct {
	// All files saved in cache file.
	//
	// Note that the map contains files scanned
	// by previous scanning, and the map is a super set.
	//
	// The key is file full path, would be lower case on Windows.
	cacheFiles map[string]*FileAttr

	// All files scanned this time.
	scannedFiles map[SHA256Digest][]*FileAttr

	paths        []string  // Source paths to scan
	filter       Filter    // Filter.
	updater      Updater   // Updater interface
	cache        string    // Cache file path.
	totalFiles   int       // Total files (map scannedFiles).
	totalFolders int       // Total folders.
	totalBytes   int64     // Total size (map scannedFiles), in bytes.
	hashEngine   hash.Hash // SHA256 hash engine.
	buffer       []byte    // Buffer for reading file content.
	cacheDirty   bool      // Indicates if cache file needs to update.
}

// Create a new file scanner.
func NewFileScanner(paths []string,
	filter Filter, updater Updater) FileScanner {

	return &fileScannerImpl{
		cacheFiles:   make(map[string]*FileAttr),
		scannedFiles: make(map[SHA256Digest][]*FileAttr),
		paths:        paths,
		filter:       filter,
		updater:      updater,
		cache:        (filter.GetCacheDir() + string(os.PathSeparator) + "global.cache"),
		hashEngine:   sha256.New(),
		buffer:       make([]byte, 512*1024),
	}
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

func (me *fileScannerImpl) GetScannedFiles() map[SHA256Digest][]*FileAttr {
	return me.scannedFiles
}

func (me *fileScannerImpl) OnFileRemoved(removed *FileAttr) {
	delete(me.cacheFiles, GetPathAsKey(removed.Path))
	me.cacheDirty = true
}

func (me *fileScannerImpl) Scan() error {
	for _, path := range me.paths {
		// Save old numbers.
		oldTotalFiles := me.totalFiles
		oldTotalFolders := me.totalFolders
		oldTotalBytes := me.totalBytes

		// Start to scan this path.
		me.updater.Log(LOG_INFO, "Scanning %v...", path)

		// Get path attribute.
		info, err := os.Stat(path)
		if err != nil {
			me.updater.IncreaseErrors()
			me.updater.Log(LOG_ERROR, "%v (%v)", err, path)
			me.updater.Log(LOG_INFO, "")
			continue
		}

		// Check if the path needs to skip.
		if !me.filter.Skip(path, info.Name(), info.IsDir()) {
			if info.IsDir() {
				if err := me.scanFolder(path); err != nil {
					return err
				}
			} else {
				me.scanFile(path, info)
			}
		}

		// Print summary for this path.
		me.updater.Log(LOG_INFO, "%v files, %v folders, %.3f MB",
			me.totalFiles-oldTotalFiles,
			me.totalFolders-oldTotalFolders,
			float64(me.totalBytes-oldTotalBytes)/(1024*1024))
		me.updater.Log(LOG_INFO, "")
	}

	return nil
}

// Scan folder and all its sub-folders.
func (me *fileScannerImpl) scanFolder(path string) error {

	var head, tail int = 0, 1
	folders := make([]string, 0, 64)
	folders = append(folders, path)

	for head < tail {
		// Check if fatal error ever happened.
		if err := me.updater.FatalError(); err != nil {
			return err
		}

		// Pop a folder path.
		folder := folders[head]
		head++

		if len(folder) > len(path) {
			me.updater.Log(LOG_INFO, "Scanning %v...", folder)
		}

		// Open this folder.
		fp, err := os.Open(folder)
		if err != nil {
			me.updater.IncreaseErrors()
			me.updater.Log(LOG_ERROR, "Could not open folder %v. Error:%v", folder, err)
			continue
		}

		for {
			items, errReadDir := fp.Readdir(512)
			if errReadDir != nil && errReadDir != io.EOF {
				me.updater.IncreaseErrors()
				me.updater.Log(LOG_ERROR, "Could not enumerate folder %v. Error:%v", folder, errReadDir)
				break
			}

			for i := 0; i < len(items); i++ {
				// Check if fatal error ever happened.
				if err := me.updater.FatalError(); err != nil {
					// Close the folder
					fp.Close()
					return err
				}

				subPath := AppendPath(folder, items[i].Name())

				// Check if it needs to skip.
				if me.filter.Skip(subPath, items[i].Name(), items[i].IsDir()) {
					continue
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

func (me *fileScannerImpl) onFileFound(newFile *FileAttr) {
	// Update map[SHA256]...
	if list, ok := me.scannedFiles[newFile.SHA256]; ok {
		for _, existing := range list {
			// 1. Check file size here to avoid hash collision.
			// 2. If the two paths are the same, then skip.
			// 3. If the two paths point to the same file, then skip.
			if existing.Size != newFile.Size ||
				SamePath(existing.Path, newFile.Path) ||
				os.SameFile(existing.Details, newFile.Details) {
				return
			}
		}

		me.scannedFiles[newFile.SHA256] = append(list, newFile)
	} else {
		me.scannedFiles[newFile.SHA256] = []*FileAttr{newFile}
	}

	me.updater.Log(LOG_TRACE, "%v (%v)", newFile.Path, &newFile.SHA256)

	// Update total count.
	me.totalFiles++
	me.totalBytes += newFile.Size
}

// Calculate file checksum and put it to the map.
func (me *fileScannerImpl) scanFile(
	path string, info os.FileInfo) error {

	// File path is map key.
	key := GetPathAsKey(path)

	// If the file already exists in the map,
	// and file size & last modification time are the same,
	// then skip to read file content to enhance performance.
	if value, found := me.cacheFiles[key]; found {
		if value.Size == info.Size() && value.ModTime == info.ModTime().UnixNano() {
			// Set FileAttr.Details to valid value.
			value.Details = info

			// Update total count and map[SHA256]...
			me.onFileFound(value)

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

	me.updater.Log(LOG_TRACE, "Calculating checksum for %v...", path)

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
	me.cacheFiles[key] = newValue

	// A new file was added, set dirty flag to true.
	me.cacheDirty = true

	// Update total count and map[SHA256]...
	me.onFileFound(newValue)

	return nil
}

func (me *fileScannerImpl) ReadCache() error {
	// Print trace log message.
	me.updater.Log(LOG_TRACE, "Reading cache %v...", me.cache)

	// Open cache file.
	fp, err := os.Open(me.cache)
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

		if err := object.ReadCache(reader); err == nil {
			me.cacheFiles[GetPathAsKey(object.Path)] = object
		} else if err == io.EOF {
			break
		} else {
			return err
		}
	}

	return nil
}

func (me *fileScannerImpl) SaveCache() error {
	if !me.cacheDirty {
		return nil
	}

	me.cacheDirty = false

	// Create cache folder if it does not exist.
	if _, err := os.Stat(me.filter.GetCacheDir()); err != nil {
		if err := os.Mkdir(me.filter.GetCacheDir(), os.ModePerm); err != nil {
			return err
		}
	}

	// Print trace log message.
	me.updater.Log(LOG_TRACE, "Updating cache %v...", me.cache)

	// Create a new cache file.
	fp, err := os.OpenFile(me.cache, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer fp.Close()

	// Create a buffered writer to enhance performance.
	writer := bufio.NewWriter(fp)

	// Write all files with their hashes to disk.
	for _, object := range me.cacheFiles {
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
