// File deduplication
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Return value of promptDelete()
const (
	PROMPT_ANSWER_YES = iota
	PROMPT_ANSWER_NO
	PROMPT_ANSWER_ALL
	PROMPT_ANSWER_QUIT
)

func usage() {
	fmt.Println("Copyright 2015 (C) Alex Jin (toalexjin@hotmail.com)")
	fmt.Println("Remove duplicated files from your system.")
	fmt.Println()
	fmt.Println("Usage: dedup [-v] [-f] [-t] [-p <policy>,...] <path>...")
	fmt.Println()
	fmt.Println("Options and Arguments:")
	fmt.Println("    -v:        Verbose mode.")
	fmt.Println("    -f:        Do not prompt before removing files.")
	fmt.Println("    -t:        Show duplicated files, do not delete them.")
	fmt.Println("    -p:        Policy indicates which files to remove.")
	fmt.Println()
	fmt.Println("<policy>:")
	fmt.Println("    longname:  Remove duplicated files with longer file name.")
	fmt.Println("    shortname: Remove duplicated files with shorter file name.")
	fmt.Println("    longpath:  Remove duplicated files with longer full path.")
	fmt.Println("    shortpath: Remove duplicated files with shorter full path.")
	fmt.Println("    new:       Remove duplicated files with newer last modification time.")
	fmt.Println("    old:       Remove duplicated files with older last modification time.")
	fmt.Println()
	fmt.Println("Default <policy>: \"longname,longpath,new\"")
	fmt.Println()
}

// Input paths might be relative and duplicated,
// we need to convert to absolute paths and remove duplicated.
func getAbsUniquePaths(paths []string) ([]string, error) {

	// For storing unique paths.
	uniquePaths := make([]string, 0, len(paths))

	for _, path := range paths {
		// First, convert to absolute path.
		abs, err := GetAbsPath(path)
		if len(abs) == 0 && err == nil {
			err = ErrRootPathNotPermitted
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid argument %v (%v)\n", path, err)
			return nil, err
		}

		// Second, check if it's parent (or child) folder
		// of a path in the array.
		var i int
		for i = 0; i < len(uniquePaths); i++ {
			if SameOrIsChild(uniquePaths[i], abs) {
				break
			} else if SameOrIsChild(abs, uniquePaths[i]) {
				uniquePaths[i] = abs
				break
			}
		}

		if i == len(uniquePaths) {
			uniquePaths = append(uniquePaths, abs)
		}
	}

	return uniquePaths, nil
}

func viewFile(file string) error {
	cmd := exec.Command("explorer.exe", file)
	return cmd.Start()
}

var extentions = []string{
	"bat",
	"com",
	"dll",
	"drv",
	"exe",
	"sys",
}

func supportView(ext string) bool {
	lower := strings.ToLower(ext)

	for i := 0; i < len(extentions); i++ {
		if extentions[i] == lower {
			return false
		}
	}

	return true
}

// Return value is PROMPT_ANSWER_???
func promptDelete(file string) int {

	// Support "view" or not.
	viewFlag := false

	if os.PathSeparator != '/' {
		if supportView(filepath.Ext(file)) {
			viewFlag = true
		}
	}

	// Create a buffered reader.
	reader := bufio.NewReader(os.Stdin)

	for {
		if viewFlag {
			fmt.Printf("Delete %v? (Yes,All,No,View,Quit):", file)
		} else {
			fmt.Printf("Delete %v? (Yes,All,No,Quit):", file)
		}

		if line, _, err := reader.ReadLine(); err == nil {

			switch strings.ToLower(string(line)) {
			case "y", "yes":
				return PROMPT_ANSWER_YES

			case "n", "no":
				return PROMPT_ANSWER_NO

			case "a", "all":
				return PROMPT_ANSWER_ALL

			case "v", "view":
				if viewFlag {
					viewFile(file)
				}

			case "q", "quit":
				return PROMPT_ANSWER_QUIT

			default:
				if len(line) > 0 {
					fmt.Fprintf(os.Stderr, "Invalid Command: %v\n\n", string(line))
				}
			}
		}
	}
}

// Map SHA256 hash to file.
//
// map[SHA256]HashItem
type HashItem struct {
	File    *FileAttr   // File attributes.
	Scanner FileScanner // Belongs to which file scanner
}

func main_i() int {

	var verbose bool
	var force bool
	var test bool
	var policySpec string

	// Parse command line options.
	flag.BoolVar(&verbose, "v", false, "Verbose mode.")
	flag.BoolVar(&force, "f", false, "Do not prompt before removing files.")
	flag.BoolVar(&test, "t", false, "Show duplicated files, do not delete them.")
	flag.StringVar(&policySpec, "p", "", "Policy indicates which files to remove.")
	flag.Parse()

	// If argument is missing, then exit.
	if flag.NArg() == 0 {
		usage()
		return 1
	}

	// Create a policy to determine which file to delete.
	policy, err := NewPolicy(policySpec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	// Convert input paths to absolute.
	paths, err := getAbsUniquePaths(flag.Args())
	if err != nil {
		return 1
	}

	// Create a updater callback object.
	updater := NewUpdater(verbose)

	// Create a filter object.
	filter := NewFilter()

	// Create file scanner for each path.
	scanners := make([]FileScanner, 0, len(paths))

	for i := 0; i < len(paths); i++ {
		info, err := os.Stat(paths[i])
		if err != nil {
			updater.Log(LOG_ERROR, "%v (%v)", err, paths[i])
			return 1
		}

		scanners = append(scanners,
			NewFileScanner(paths[i], info, filter, updater))
	}

	// Scan files.
	for _, scanner := range scanners {
		updater.Log(LOG_INFO, "Scanning %v...", scanner.GetPath())

		// Ignore error because cache is not very important.
		scanner.ReadCache()

		if err := scanner.Scan(); err != nil {
			return 1
		}

		updater.Log(LOG_INFO, "%v files, %v folders, %.3f MB",
			scanner.GetTotalFiles(), scanner.GetTotalFolders(),
			float64(scanner.GetTotalBytes())/(1024*1024))
		updater.Log(LOG_INFO, "")
	}

	// Result variables
	var totalFiles int = 0
	var totalFolders int = 0
	var totalBytes int64 = 0
	var deletedFiles int = 0
	var deletedBytes int64 = 0

	// Map hash-value to file attribute & scanner.
	mapping := make(map[SHA256Digest]HashItem)

	// Iterate scanner one by one.
	for _, scanner := range scanners {
		totalFiles += scanner.GetTotalFiles()
		totalFolders += scanner.GetTotalFolders()
		totalBytes += scanner.GetTotalBytes()

		// Iterate file one by one.
		for _, file := range scanner.GetFiles() {
			// Find if the hash already exists in the map.
			if existing, found := mapping[file.SHA256]; !found {
				// It is a new hash.
				mapping[file.SHA256] = HashItem{
					File: file, Scanner: scanner,
				}
			} else {
				// The hash already exists.
				var deleted, remain HashItem
				var goAhead bool = true

				// Check which file to remove
				switch policy.DeleteWhich(existing.File, file) {
				case DELETE_WHICH_FIRST:
					deleted = existing
					remain.File = file
					remain.Scanner = scanner

				case DELETE_WHICH_EITHER:
					fallthrough

				case DELETE_WHICH_SECOND:
					deleted.File = file
					deleted.Scanner = scanner
					remain = existing

				case DELETE_WHICH_NEITHER:
					goAhead = false
				}

				// Prompt before remove file.
				if !test && goAhead && !force {
					switch promptDelete(deleted.File.Path) {
					case PROMPT_ANSWER_YES:

					case PROMPT_ANSWER_ALL:
						force = true

					case PROMPT_ANSWER_NO:
						goAhead = false

					case PROMPT_ANSWER_QUIT:
						goAhead = false

						// Update cache files.
						for i := 0; i < len(scanners); i++ {
							scanners[i].SaveCache()
						}

						return 1
					}
				}

				// Remove the file.
				if goAhead {
					// Remove the item and update hash map.
					if existing.File.Path != deleted.File.Path {
						mapping[file.SHA256] = HashItem{
							File: file, Scanner: scanner,
						}
					}

					if test {
						// Show duplicated files, do not delete them.
						//
						// Be aware that we do not remove dupliated files
						// from the map because we want to save their
						// SHA256 hashes in local cache.
						updater.Log(LOG_INFO, "%v is duplicated (%v).",
							deleted.File.Path, remain.File.Path)

						deletedBytes += deleted.File.Size
						deletedFiles++
					} else {
						// Delete duplicated file from the map.
						deleted.Scanner.Remove(deleted.File.Path)

						// Delete duplicated file from disk.
						if err := os.Remove(deleted.File.Path); err != nil {
							updater.Log(LOG_ERROR, "Could not delete file %v (%v).",
								deleted.File.Path, err)
							updater.IncreaseErrors()
						} else {
							updater.Log(LOG_INFO, "File %v was deleted.", deleted.File.Path)
							deletedBytes += deleted.File.Size
							deletedFiles++
						}
					}
				}
			}
		}
	}

	// If a folder has changes, then update its local cache.
	for _, scanner := range scanners {
		scanner.SaveCache()
	}

	if deletedFiles > 0 {
		updater.Log(LOG_INFO, "")
	}

	updater.Log(LOG_INFO, "<Summary>")

	if test {
		updater.Log(LOG_INFO, "Duplicated Files: %v", deletedFiles)
		updater.Log(LOG_INFO, "Duplicated Size:  %.3f MB", float64(deletedBytes)/(1024*1024))
	} else {
		updater.Log(LOG_INFO, "Deleted Files:    %v", deletedFiles)
		updater.Log(LOG_INFO, "Deleted Size:     %.3f MB", float64(deletedBytes)/(1024*1024))
	}

	if updater.Errors() > 0 {
		updater.Log(LOG_INFO, "Errors:        %v", updater.Errors())
	}

	return 0
}

func main() {
	result := main_i()

	os.Exit(result)
}
