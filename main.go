package main

import (
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
		panic("source directory not specified")
	}

	if !DirExists(*sourceDir) {
		panic("source directory does not exist")
	}

	if outDir == nil {
		panic("out directory not specified")
	}

	if err := CreateDirIfNotExists(*outDir); err != nil {
		panic(err)
	}

	if err := sortPhotos(*sourceDir, *outDir); err != nil {
		panic(err)
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

		// get file extension
		//fileExt := fileInfo.Name()[len(fileInfo.Name())-4:]
		//if fileExt != ".jpg" && fileExt != ".png" {
		//	continue
		//}

		log.Printf("processing file %s", entry.Name())

		fileCreationDate := fileInfo.ModTime()
		fileCreationYear := fileCreationDate.Year()
		fileCreationMonth := fileCreationDate.Month()

		log.Printf("file %s created on %d-%02d", entry.Name(), fileCreationYear, fileCreationMonth)

		yearDir := fmt.Sprintf("%s/%d", outDir, fileCreationYear)
		if err := CreateDirIfNotExists(yearDir); err != nil {
			return fmt.Errorf("create year directory %s: %v", yearDir, err)
		}

		monthDir := fmt.Sprintf("%s/%d-%02d", yearDir, fileCreationYear, fileCreationMonth)
		if err := CreateDirIfNotExists(monthDir); err != nil {
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
