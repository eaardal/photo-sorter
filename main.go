package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
)

var sourceDir = flag.String("source", "", "Source directory")
var outDir = flag.String("out", "", "Output directory")

func main() {
	flag.Parse()

	if sourceDir == nil {
		log.Fatal("source directory not specified")
	}

	if !dirExists(*sourceDir) {
		log.Fatal("source directory does not exist")
	}

	if outDir == nil {
		log.Fatal("out directory not specified")
	}

	if err := createDirIfNotExists(*outDir); err != nil {
		log.Fatal(err)
	}

	if err := sortPhotos(*sourceDir, *outDir); err != nil {
		log.Fatal(err)
	}
}

// sortPhotos will read all picture files from source directory and sort them by month and year under the out directory.
func sortPhotos(sourceDir string, outDir string) error {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("read source dir %s: %v", sourceDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileInfo, err := entry.Info()
		if err != nil {
			return fmt.Errorf("get file info for %s: %v", entry.Name(), err)
		}

		if !isPicture(fileInfo) {
			log.Printf("file %s is not a picture, skipping", entry.Name())
			continue
		}

		log.Printf("processing file %s", entry.Name())

		fileCreationDate := fileInfo.ModTime()
		fileCreationYear := fileCreationDate.Year()
		fileCreationMonth := fileCreationDate.Month()

		log.Printf("file %s created on %d-%02d", entry.Name(), fileCreationYear, fileCreationMonth)

		yearDir := fmt.Sprintf("%s/%d", outDir, fileCreationYear)
		if err := createDirIfNotExists(yearDir); err != nil {
			return fmt.Errorf("create year directory %s: %v", yearDir, err)
		}

		monthDir := fmt.Sprintf("%s/%d-%02d", yearDir, fileCreationYear, fileCreationMonth)
		if err := createDirIfNotExists(monthDir); err != nil {
			return fmt.Errorf("create month directory %s: %v", monthDir, err)
		}

		fileContent, err := os.ReadFile(fmt.Sprintf("%s/%s", sourceDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read file %s: %v", entry.Name(), err)
		}

		outPath := fmt.Sprintf("%s/%s", monthDir, entry.Name())
		if err := os.WriteFile(outPath, fileContent, 0644); err != nil {
			return fmt.Errorf("write file %s: %v", entry.Name(), err)
		}

		log.Printf("file %s copied to %s", entry.Name(), outPath)
	}

	return nil
}

func isPicture(fileInfo os.FileInfo) bool {
	return true // Just sort all files for now

	//fileExt := fileInfo.Name()[len(fileInfo.Name())-4:]
	//return fileExt == ".jpg" || fileExt == ".png"
}

func dirExists(path string) bool {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return false
	}
	return true
}

func createDirIfNotExists(path string) error {
	err := os.Mkdir(path, 0777)
	if err == nil {
		return nil
	}

	if os.IsExist(err) {
		stat, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("os.Stat: failed to read %s: %v", path, err)
		}

		if !stat.IsDir() {
			return fmt.Errorf("path %s exists but is not a directory", path)
		}
		return nil
	}

	return err
}
