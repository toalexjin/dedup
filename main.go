// File deduplication
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Return value of promptKeep()
const (
	// Keep the first one and remove the rest.
	PROMPT_ANSWER_YES = iota

	// Do not remove duplicated files this time,
	// will prompt again when duplication happens next time.
	PROMPT_ANSWER_SKIP

	// Do not prompt again and remove all duplicated files.
	PROMPT_ANSWER_CONTINUE

	// Abort program.
	PROMPT_ANSWER_QUIT
)

func usage() {
	fmt.Println("Copyright 2015 (C) Alex Jin (toalexjin@hotmail.com)")
	fmt.Println("Remove duplicated files from your system.")
	fmt.Println()
	fmt.Println("Usage: dedup [-v] [-f] [-l] [-i <TYPE>,...] [-e <TYPE>,...] [-p <POLICY>,...] <path>...")
	fmt.Println()
	fmt.Println("Options and Arguments:")
	fmt.Println("    -v:        Verbose mode.")
	fmt.Println("    -f:        Do not prompt before removing each duplicated file.")
	fmt.Println("    -l:        List duplicated files only, do not remove them.")
	fmt.Println("    -i:        Include filters (Scan & remove specified files only).")
	fmt.Println("    -e:        Exclude filters (Do NOT scan & remove specified files).")
	fmt.Println("    -p:        When duplication happens, which file will be removed.")
	fmt.Println()
	fmt.Println("-i <TYPE>, -e <TYPE>:")
	fmt.Println("    audio:     Audio files.")
	fmt.Println("    office:    Microsoft Office documents.")
	fmt.Println("    photo:     Photo (picture) files.")
	fmt.Println("    video:     Video files.")
	fmt.Println("    tarball:   Compressed and ISO files (e.g. gz, iso, rar, zip).")
	fmt.Println()
	fmt.Println("    Remark: If both include and exclude filters are not set,")
	fmt.Println("            then all files will be scanned.")
	fmt.Println()
	fmt.Println("-p <POLICY>:")
	fmt.Println("    longname:  Remove duplicated files with longer file name.")
	fmt.Println("    shortname: Remove duplicated files with shorter file name.")
	fmt.Println("    longpath:  Remove duplicated files with longer full path.")
	fmt.Println("    shortpath: Remove duplicated files with shorter full path.")
	fmt.Println("    new:       Remove duplicated files with newer last modification time.")
	fmt.Println("    old:       Remove duplicated files with older last modification time.")
	fmt.Println()
	fmt.Println("    Remark: If \"-p <POLICY>\" is not set, then default policy")
	fmt.Println("            \"longname,longpath,new\" will be used.")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("    > dedup -l d:\\data e:\\data")
	fmt.Println("      List duplicated files.")
	fmt.Println()
	fmt.Println("    > dedup -i photo,video d:\\data e:\\data")
	fmt.Println("      Remove duplicated photo & video.")
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
			if _, err := os.Stat(abs); err != nil {
				fmt.Fprintf(os.Stderr, "%v (%v)", err, path)
				return nil, err
			}

			uniquePaths = append(uniquePaths, abs)
		}
	}

	return uniquePaths, nil
}

func showDuplicatedFiles(files []*FileAttr) {
	fmt.Printf("* 1) %v\n", files[0].Path)

	for i := 1; i < len(files); i++ {
		fmt.Printf("  %v) %v\n", i+1, files[i].Path)
	}
}

func viewFile(file string) error {
	cmd := exec.Command("explorer.exe", file)
	return cmd.Start()
}

// Return value is PROMPT_ANSWER_???
//
// Note that this function might modify input slice "files".
// If return value is PROMPT_ANSWER_YES or PROMPT_ANSWER_CONTINUE,
// the first file in slice "files" need to keep and the rest
// of files need to remove.
func promptKeep(files []*FileAttr) int {

	numberStr := "1"
	for i := 1; i < len(files); i++ {
		numberStr += fmt.Sprintf(",%v", i+1)
	}

	// Create a buffered reader.
	reader := bufio.NewReader(os.Stdin)

	for {
		// Print duplicated files.
		showDuplicatedFiles(files)

		fmt.Printf("Which file do you want to keep? (1-%v,Skip,Continue,Quit):", len(files))
		if line, _, err := reader.ReadLine(); err == nil {
			cmd := strings.ToLower(string(line))

			switch cmd {
			case "s", "skip":
				return PROMPT_ANSWER_SKIP

			case "c", "continue":
				return PROMPT_ANSWER_CONTINUE

			case "q", "quit":
				return PROMPT_ANSWER_QUIT

			default:
				if len(line) > 0 {
					index, err := strconv.Atoi(cmd)
					if err != nil || index < 1 || index > len(files) {
						fmt.Fprintf(os.Stderr, "Invalid Command: %v\n\n", string(line))
					} else {
						if index != 1 {
							tmp := files[0]
							files[0] = files[index-1]
							files[index-1] = tmp
						}

						return PROMPT_ANSWER_YES
					}
				} else {
					fmt.Println()
				}
			}
		}
	}
}

func main_i() int {

	var verbose bool
	var force bool
	var list bool
	var includes string
	var excludes string
	var policySpec string

	// Parse command line options.
	flag.BoolVar(&verbose, "v", false, "Verbose mode.")
	flag.BoolVar(&force, "f", false, "Do not prompt before removing files.")
	flag.BoolVar(&list, "l", false, "List duplicated files only, do not remove them.")
	flag.StringVar(&includes, "i", "", "Include filters.")
	flag.StringVar(&excludes, "e", "", "Exclude filters.")
	flag.StringVar(&policySpec, "p", "", "Policy indicates which files to remove.")
	flag.Parse()

	// If argument is missing, then exit.
	if flag.NArg() == 0 {
		usage()
		return 1
	}

	// Create policy object to determine
	// which file to delete when duplication happens.
	policy, err := NewPolicy(policySpec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	// Create filter object.
	filter, err := NewFilter(includes, excludes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	// Convert input paths to absolute.
	paths, err := getAbsUniquePaths(flag.Args())
	if err != nil {
		return 1
	}

	// Create status updater.
	updater := NewUpdater(verbose)

	// Create file scanner.
	scanner := NewFileScanner(paths, filter, updater)

	// Ignore error because cache is not very important.
	scanner.ReadCache()

	// Scan files.
	if err := scanner.Scan(); err != nil {
		return 1
	}

	// Result variables
	var deletedFiles int = 0
	var deletedBytes int64 = 0
	var first_prompt = true

	// Iterate all scanned files.
	for _, item := range scanner.GetScannedFiles() {
		// If no duplicated files, then skip.
		if len(item) <= 1 {
			continue
		}

		// Once returned, item[0] needs to keep
		// and the rest could be removed.
		policy.Sort(item)

		if list {
			showDuplicatedFiles(item)

			deletedFiles += len(item) - 1
			for i := 1; i < len(item); i++ {
				deletedBytes += item[i].Size
			}
		} else {
			if !force {
				if first_prompt {
					first_prompt = false
				} else {
					fmt.Println()
				}

				// Prompt before remove file.
				if result := promptKeep(item); result == PROMPT_ANSWER_SKIP {
					continue
				} else if result == PROMPT_ANSWER_QUIT {
					scanner.SaveCache()
					return 1
				} else if result == PROMPT_ANSWER_CONTINUE {
					force = true
				}
			}

			// Delete duplicated files, range [1,len).
			for i := 1; i < len(item); i++ {
				if err := os.Remove(item[i].Path); err != nil {
					updater.Log(LOG_ERROR, "Could not delete file %v (%v).",
						item[i].Path, err)
					updater.IncreaseErrors()
					continue
				}

				// Write log and update file count.
				updater.Log(LOG_INFO, "%v was deleted.", item[i].Path)
				deletedBytes += item[i].Size
				deletedFiles++

				// Update cache file.
				scanner.OnFileRemoved(item[i])
			}
		}
	}

	// Update local cache.
	scanner.SaveCache()

	if deletedFiles > 0 {
		updater.Log(LOG_INFO, "")
	}

	updater.Log(LOG_INFO, "<Summary>")
	updater.Log(LOG_INFO, "Total Files:      %v", scanner.GetTotalFiles())
	updater.Log(LOG_INFO, "Total Folders:    %v", scanner.GetTotalFolders())
	updater.Log(LOG_INFO, "Total Size:       %.3f MB", float64(scanner.GetTotalBytes())/(1024*1024))

	if list {
		updater.Log(LOG_INFO, "Duplicated Files: %v", deletedFiles)
		updater.Log(LOG_INFO, "Duplicated Size:  %.3f MB", float64(deletedBytes)/(1024*1024))
	} else {
		updater.Log(LOG_INFO, "Deleted Files:    %v", deletedFiles)
		updater.Log(LOG_INFO, "Deleted Size:     %.3f MB", float64(deletedBytes)/(1024*1024))
	}

	if updater.Errors() > 0 {
		updater.Log(LOG_INFO, "Errors:           %v", updater.Errors())
	}

	return 0
}

func main() {
	result := main_i()

	os.Exit(result)
}
