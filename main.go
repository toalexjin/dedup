// File deduplication
package main

import (
	"flag"
	"fmt"
	"os"
)

func usage() {
	fmt.Println("Remove duplicated files from your system.")
	fmt.Println()
	fmt.Println("Usage: dedup [-f] [-p <policy>,...] <path>...")
	fmt.Println()
	fmt.Println("Options and Arguments:")
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
func getAbsoleteUniquePaths(paths []string) {

	// To-do:

	return paths
}

type MappingItem struct {
	File    *FileAttr    // File attributes.
	Scanner *FileScanner // Belongs to which file scanner
}

func main_i() int {

	var force bool
	var policySpec string

	// Parse command line options.
	flag.BoolVar(&force, "f", false, "Do not prompt before removing files.")
	flag.StringVar(&policySpec, "p", "", "Policy indicates which files to remove.")
	flag.Parse()

	// If argument is missing, then exit.
	if flag.NArg() == 0 {
		usage()
		return 1
	}

	// Create file scanner for each path.
	paths := getAbsoleteUniquePaths(flag.Args())
	scanners := make([]*FileScanner, len(paths))

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v (%v)", err, path)
			return 1
		}

		scanners[i] = NewFileScanner(path, info)
	}

	// Scan files.
	for _, scanner := range scanners {
		if err := scanner.Scan(); err != nil {
			return 1
		}
	}

	// Create a policy object to determine which file to delete.
	policy := NewPolicy(policySpec)

	// Result variables
	var totalFiles int64 = 0
	var totalFolders int64 = 0
	var totalBytes int64 = 0
	var deletedFiles int64 = 0
	var deletedBytes int64 = 0
	var errors int64 = 0
	var aborted bool = false

	// Map hash-value to file attribute & scanner.
	mapping := make(map[string]MappingItem)

	// Iterate scanner one by one.
	for i := 0; i < len(scanners); i++ {
		totalFiles += scanners[i].GetTotalFiles()
		totalFolders += scanners[i].GetTotalFolders()
		totalBytes += scanners[i].GetTotalBytes()

		// Iterate file one by one.
		for _, file := range scanners[i].GetFiles() {
			// Get this file's hash value.
			key := file.GetHashKey()

			// Find if the hash already exists in the map.
			if existing, found := mapping[key]; !found {
				// It is a new hash.
				mapping[key] = MappingItem{
					File: file, Scanner: scanners[i],
				}
			} else {
				// The hash already exists.
				var deleted MappingItem
				var goAhead bool = true

				// Check which file to remove
				switch policy.DeleteWhich(existing.File, file) {
				case DELETE_WHICH_FIRST:
					deleted = existing

				case DELETE_WHICH_EITHER:
					fallthrough

				case DELETE_WHICH_SECOND:
					deleted.File = file
					deleted.Scanner = scanners[i]

				case DELETE_WHICH_NEITHER:
					goAhead = false
				}

				// Prompt before remove file.
				if goAhead && !force {
					switch PromptDelete(deleted.File.Path) {
					case PROMPT_YES_ALL:
						force = true

					case PROMPT_NO:
						goAhead = false

					case PROMPT_ABORT:
						goAhead = false
						aborted = true
						break abortedLabel
					}
				}

				// Remove the file.
				if goAhead {
					deleted.Scanner.Remove(deleted.File.Path)
					if existing.File.Path != deleted.File.Path {
						mapping[key] = MappingItem{
							File: file, Scanner: scanners[i],
						}
					}

					// Delete this file.
					if err := os.Remove(deleted.File.Path); err != nil {
						fmt.Fprintf(os.Stderr, "Could not delete file %v (%v)",
							deleted.File.Path, err)
						errors++
					} else {
						deletedBytes += deleted.File.Size
						deletedFiles++
					}
				}
			}
		}
	}

	fmt.Println()
	fmt.Printf("Total Files:   %v\n", totalFiles)
	fmt.Printf("Total Folders: %v\n", totalFolders)
	fmt.Printf("Total Size:    %v MB\n", totalBytes/(1024*1024))
	fmt.Printf("Deleted Files: %v\n", deletedFiles)
	fmt.Printf("Deleted Size:  %v MB\n", deletedBytes/(1024*1024))

	if errors > 0 {
		fmt.Printf("Errors:        %v\n", errors)
	}

abortedLabel:

	// If a folder has changes, then update its local cache.
	for _, scanner := range scanners {
		if scanner.IsDirty() {
			scanner.Save()
		}
	}

	return 0
}

func main() {
	result := main_i()

	os.Exit(result)
}
