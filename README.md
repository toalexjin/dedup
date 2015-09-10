# Dedup

This project is to remove duplicated files from your system.
For instance, removing duplicated pictures to free disk space.

## Usage

```
dedup [-f] [-p <policy,...>] <path>...
```

**Options and Arguments:**

- `-f`: Do not prompt before removing files.
- `-p <policy,...>`:
    - **new**: Remove duplicated files with newer last modification time.
    - **old**: Remove duplicated files with older last modification time.
    - **longname**: Remove duplicated files with longer file name.
    - **shortname**: Remove duplicated files with shorter file name.
    - **longpath**: Remove duplicated files with longer full path.
    - **shortpath**: Remove duplicated files with shorter full path.
- `<path>...`:  One or multiple file paths to scan.

**Remark**:

- If option `-p <policy,...>` is not specified, then default policy
  `-p new,longname,longpath` will be used.

## Supported Platforms

It's written in Go language, which is platform independent.
That's to say, almost all platforms are supported,
e.g. Windows, Linux, Mac,...

## Build

1. Install latest **Go Compiler** (https://golang.org)
2. Set environment variable `$GOROOT=/path/to/go` and add it to $PATH.
3. git clone https://github.com/toalexjin/dedup.git
4. cd dedup
5. go build
6. ./dedup ...
