clean-dist:
	rm -rf ./dist

build:
	go build -o photosorter.exe ./...

build-windows:
	GOOS=windows GOARCH=amd64 go build -o ./dist/windows/photosorter.exe ./...

build-linux:
	GOOS=linux GOARCH=amd64 go build -o ./dist/linux/photosorter ./...

build-macos-intel:
	GOOS=darwin GOARCH=amd64 go build -o ./dist/macos-intel/photosorter ./...

build-macos-apple-silicon:
	GOOS=darwin GOARCH=arm64 go build -o ./dist/macos-apple-silicon/photosorter ./...

dist: clean-dist build-windows build-linux build-macos-intel build-macos-apple-silicon
	cd ./dist && zip -r binaries.zip linux/ macos-intel/ macos-apple-silicon/ windows/
