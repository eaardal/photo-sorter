# Photo Sorter

When downloading photos from Google Photos, it zips everything into one folder. If the album is large, it can be cumbersome to sort and cull the files.

This utility will take all files in the given source directory and sort them into sub-folders by month and year.

The files are also sorted into _pictures_ and _videos_ sub-folders based on the file's extension.

## Usage

```shell
make build && photosorter --source <source> --out <destination>
# OR
go run main.go --source <source> --out <destination>
```

Build binary:

```shell
make build
```

Build binaries for release:

```shell
make dist
```