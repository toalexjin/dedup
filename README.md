# Dedup

This project is to remove duplicated files from your system.
For instance, removing duplicated pictures to free disk space.

## Usage

```
dedup [-v] [-f] [-l] [-i <TYPE,...>] [-e <TYPE,...>] [-p <POLICY,...>] <path>...
```

**Options and Arguments:**

- `-v`: Verbose mode.
- `-f`: Do not prompt before removing each duplicated file.
- `-l`: List duplicated files only, do not remove them.
- `-i <TYPE,...>`: Include filters (Scan & remove specified files only).
- `-e <TYPE,...>`: Exclude filters (Do NOT scan & remove specified files).
- `-p <POLICY,...>`: When duplication happens, which file will be removed.
- `<TYPE,...>`
    - **audio**: Audio files.
    - **office**: Microsoft Office documents.
    - **photo**: Photo (picture) files.
    - **video**: Video files.
    - **package**: Tarball, compressed, ISO, installation packages, etc.
- `<POLICY,...>`
    - **longname**: Remove duplicated files with longer file name.
    - **shortname**: Remove duplicated files with shorter file name.
    - **longpath**: Remove duplicated files with longer full path.
    - **shortpath**: Remove duplicated files with shorter full path.
    - **new**: Remove duplicated files with newer last modification time.
    - **old**: Remove duplicated files with older last modification time.
- `<path>...`:  One or multiple file paths to scan.

**Remark**:

- If both include and exclude filters are not set, then
  all duplicated files will be removed.
- If `-p <POLICY,...>` is not set, then default policy
  `-p longname,longpath,new` will be used. Be aware
  that the order of policy items is very important.

## Examples

1. `dedup d:\data e:\data`: Remove all duplicated files.
2. `dedup -f d:\data e:\data`: Do **NOT** prompt before removing each duplicated file.
3. `dedup -l d:\data e:\data`: List duplicated files only, do **NOT** remove them.
4. `dedup -i photo,video d:\data e:\data`: Remove duplicated **photo** and **video**
   files only, and do **NOT** remove any other duplicated files.
5. `dedup -e office,package d:\data e:\data`: Do not remove duplicated
   Microsoft Office documents and package files.

## Best Practice

1. You could run `dedup -l <path>` to check duplicated files before really removing them.
2. It's better to always use **Include Filters** to remove specified types of
   duplicated files, because it could avoid removing other duplicated files
   (e.g. system files, application files) that you want to keep. For instance,
   run `dedup -i photo,video <path>` to remove duplicated **Photo and Video** only.

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
2. `export GOROOT=/path/to/go/compiler`
3. `export PATH=$PATH:$GOROOT/bin`
4. `mkdir ~/go-workspace`
5. `export GOPATH=~/go-workspace`
6. `go get github.com/toalexjin/dedup`
7. `$GOPATH/bin/dedup`
