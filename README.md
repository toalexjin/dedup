# Dedup

This project is to remove duplicated files from your system.
For instance, removing duplicated pictures to free disk space.

## Usage

```
dedup [-v] [-f] [-t] [-p <policy,...>] <path>...
```

**Options and Arguments:**

- `-v`: Verbose mode.
- `-f`: Do not prompt before removing files.
- `-t`: Show duplicated files, do not delete them.
- `-p <policy,...>`:
    - **longname**: Remove duplicated files with longer file name.
    - **shortname**: Remove duplicated files with shorter file name.
    - **longpath**: Remove duplicated files with longer full path.
    - **shortpath**: Remove duplicated files with shorter full path.
    - **new**: Remove duplicated files with newer last modification time.
    - **old**: Remove duplicated files with older last modification time.
- `<path>...`:  One or multiple file paths to scan.

**Remark**:

- If option `-p <policy,...>` is not specified, then default policy
  `-p longname,longpath,new` will be used.

## Examples

1. `dedup d:\picture e:\picture`: Remove duplicated files.
2. `dedup -f d:\picture e:\picture`: Do not prompt before removing duplicated files.
3. `dedup -t d:\picture e:\picture`: Show duplicated files, do not delete them.

## Design

The program scans all files of specified folders, calculates SHA256 hash
for each of them. If two files have the same SHA256 hash, then the two files
would be considered as the same. To save time for calculating SHA256 hash,
the program would save all files' SHA256 hash in user home directory.
When the program runs next time, it would load the saved SHA256 hash first.
If file size and last modification time are not changed, then the program
would not calculate SHA256 hash for the file again.

## Supported Platforms

It's written in Go language, which is platform independent.
That's to say, almost all platforms are supported,
e.g. Windows, Linux, Mac,...

## Build

1. Install latest **Go Compiler** (https://golang.org)
2. `export $GOROOT=/path/to/go/compiler`
3. `export $PATH=$PATH:$GOROOT/bin`
4. `export $GOPATH=/path/to/my/code`
5. `go get github.com/toalexjin/dedup`
6. `$GOPATH/bin/dedup`
