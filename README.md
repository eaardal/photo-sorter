# Photo Sorter

When downloading photos from Google Photos, it zips everything into one folder.

This utility will sort all files into sub-folders by month and year.

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