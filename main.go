// File deduplication
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
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
	fmt.Println("Remove duplicated files from your system.")
	fmt.Println()
	fmt.Println("Usage: dedup [-v] [-f] [-p <policy>,...] <path>...")
	fmt.Println()
	fmt.Println("Options and Arguments:")
	fmt.Println("    -v:        Verbose mode.")
	fmt.Println("    -f:        Do not prompt before removing files.")
	fmt.Println("    -p:        Policy indicates which files to remove.")
	fmt.Println()
	fmt.Println("<policy>:")
	fmt.Println("    new:       Remove duplicated files with newer last modification time.")
	fmt.Println("    old:       Remove duplicated files with older last modification time.")
	fmt.Println("    longname:  Remove duplicated files with longer file name.")
	fmt.Println("    shortname: Remove duplicated files with shorter file name.")
	fmt.Println("    longpath:  Remove duplicated files with longer full path.")
	fmt.Println("    shortpath: Remove duplicated files with shorter full path.")
	fmt.Println()
	fmt.Println("Default <policy>: \"longname,new,longpath\"")
	fmt.Println()
}

// Input paths might be duplicated and might be relative paths,
// we need to remove duplicated and convert all to absolete paths.
func getAbsoleteUniquePaths(paths []string) []string {

	// TO-DO:
	//
	// Convert relative paths to absolte
	// Remove duplicated ...

	return paths
}

// Return value is PROMPT_ANSWER_???
func promptDelete(file string) int {

	// Create a buffered reader.
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("Delete %v? Yes/All/No/Quit:", file)
		if line, _, err := reader.ReadLine(); err == nil {

			switch strings.ToLower(string(line)) {
			case "y", "yes":
				return PROMPT_ANSWER_YES

			case "n", "no":
				return PROMPT_ANSWER_NO

			case "a", "all":
				return PROMPT_ANSWER_ALL

			case "q", "quit":
				return PROMPT_ANSWER_QUIT
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
	var policySpec string

	// Parse command line options.
	flag.BoolVar(&verbose, "v", false, "Verbose mode.")
	flag.BoolVar(&force, "f", false, "Do not prompt before removing files.")
	flag.StringVar(&policySpec, "p", "", "Policy indicates which files to remove.")
	flag.Parse()

	// If argument is missing, then exit.
	if flag.NArg() == 0 {
		usage()
		return 1
	}

	// Create a updater callback object.
	updater := NewUpdater(verbose)

	// Create file scanner for each path.
	paths := getAbsoleteUniquePaths(flag.Args())
	scanners := make([]FileScanner, len(paths))

	for i := 0; i < len(paths); i++ {
		info, err := os.Stat(paths[i])
		if err != nil {
			updater.Log(LOG_ERROR, "%v (%v)", err, paths[i])
			return 1
		}

		scanners[i] = NewFileScanner(paths[i], info)
	}

	// Scan files.
	for _, scanner := range scanners {
		updater.Log(LOG_INFO, "Scanning %v...", scanner.GetPath())

		if err := scanner.Scan(updater); err != nil {
			return 1
		}

		updater.Log(LOG_INFO, "%v files, %v folders, %.3f MB",
			scanner.GetTotalFiles(), scanner.GetTotalFolders(),
			float64(scanner.GetTotalBytes())/(1024*1024))
	}

	// Create a policy to determine which file to delete.
	policy, err := NewPolicy(policySpec)
	if err != nil {
		updater.Log(LOG_ERROR, "%v", err)
	}

	// Result variables
	var totalFiles int = 0
	var totalFolders int = 0
	var totalBytes int64 = 0
	var deletedFiles int = 0
	var deletedBytes int64 = 0
	var errors int = 0

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
				var deleted HashItem
				var goAhead bool = true

				// Check which file to remove
				switch policy.DeleteWhich(existing.File, file) {
				case DELETE_WHICH_FIRST:
					deleted = existing

				case DELETE_WHICH_EITHER:
					fallthrough

				case DELETE_WHICH_SECOND:
					deleted.File = file
					deleted.Scanner = scanner

				case DELETE_WHICH_NEITHER:
					goAhead = false
				}

				// Prompt before remove file.
				if goAhead && !force {
					switch promptDelete(deleted.File.Path) {
					case PROMPT_ANSWER_YES:

					case PROMPT_ANSWER_ALL:
						force = true

					case PROMPT_ANSWER_NO:
						goAhead = false

					case PROMPT_ANSWER_QUIT:
						goAhead = false
						return 1
					}
				}

				// Remove the file.
				if goAhead {
					// Remove the item and update hash map.
					deleted.Scanner.Remove(deleted.File.Path)
					if existing.File.Path != deleted.File.Path {
						mapping[file.SHA256] = HashItem{
							File: file, Scanner: scanner,
						}
					}

					// Delete the file.
					if err := os.Remove(deleted.File.Path); err != nil {
						updater.Log(LOG_ERROR, "Could not delete file %v (%v)",
							deleted.File.Path, err)
						errors++
					} else {
						updater.Log(LOG_INFO, "File %v was deleted.", deleted.File.Path)
						deletedBytes += deleted.File.Size
						deletedFiles++
					}
				}
			}
		}
	}

	// If a folder has changes, then update its local cache.
	for _, scanner := range scanners {
		scanner.Save()
	}

	updater.Log(LOG_INFO, "")
	updater.Log(LOG_INFO, "<Complete>")
	updater.Log(LOG_INFO, "Deleted Files: %v", deletedFiles)
	updater.Log(LOG_INFO, "Deleted Size:  %.3f MB", float64(deletedBytes)/(1024*1024))

	if errors > 0 {
		updater.Log(LOG_INFO, "Errors:        %v", errors)
	}

	return 0
}

func main() {
	result := main_i()

	os.Exit(result)
}
