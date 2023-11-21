package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	PicturesDirName = "pictures"
	VideosDirName   = "videos"
)

var pictureFileExtensions = []string{".jpg", ".png", ".heic", ".jpeg"}
var videoFileExtensions = []string{".mp4", ".mov"}

var sourceDirArg = flag.String("source", "", "Source directory")
var outDirArg = flag.String("out", "", "Output directory")
var fileExtensionsArg = flag.String("ext", "*", "File extensions to sort, comma separated with no spaces: \".jpg,.png\" and so on. Leave empty or '*' to sort all files")
var sortCategoriesArg = flag.Bool("categories", true, "Sort files into categories (pictures, videos)")

func main() {
	flag.Parse()

	if sourceDirArg == nil {
		log.Fatal("source directory not specified")
	}

	if !dirExists(*sourceDirArg) {
		log.Fatal("source directory does not exist")
	}

	if outDirArg == nil {
		log.Fatal("out directory not specified")
	}

	if err := createDirIfNotExists(*outDirArg); err != nil {
		log.Fatalf("failed to create out directory %s: %v", *outDirArg, err)
	}

	fileExtensions := resolveFileExtensions()

	sortCategories := true
	if sortCategoriesArg != nil {
		sortCategories = *sortCategoriesArg
	}

	if err := sortFiles(*sourceDirArg, *outDirArg, fileExtensions, sortCategories); err != nil {
		log.Fatalf("failed to sort files: %v", err)
	}
}

func sortFiles(sourceDir string, outDir string, fileExtensions []string, sortIntoCategories bool) error {
	items, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("read source dir %s: %v", sourceDir, err)
	}

	for _, item := range items {
		if item.IsDir() {
			continue
		}

		fileName := item.Name()

		fileInfo, err := item.Info()
		if err != nil {
			return fmt.Errorf("get file info for %s: %v", fileName, err)
		}

		if !shouldBeSorted(fileName, fileExtensions) {
			log.Printf("file %s does not match allowed file extensions %+v, skipping", fileName, fileExtensions)
			continue
		}

		log.Printf("copying file %s", fileName)

		outPath, err := copyFile(fileInfo, sourceDir, outDir, sortIntoCategories)
		if err != nil {
			return fmt.Errorf("copy file %s: %v", fileName, err)
		}

		if err := preserveOriginalFileCreationDate(fileInfo, outPath); err != nil {
			return fmt.Errorf("preserve original file creation date: %v", err)
		}

		log.Printf("file %s copied to %s", fileName, outPath)
	}

	return nil
}

func copyFile(fileInfo fs.FileInfo, sourceDir string, outDir string, sortIntoCategories bool) (string, error) {
	fileName := fileInfo.Name()
	fileCreationDate := fileInfo.ModTime()
	fileCreationYear := fileCreationDate.Year()
	fileCreationMonth := fileCreationDate.Month()
	fileCreationDay := fileCreationDate.Day()

	log.Printf("file %s created on %d-%02d-%02d", fileName, fileCreationYear, fileCreationMonth, fileCreationDay)

	yearDir := path.Join(outDir, fmt.Sprintf("%d", fileCreationYear))
	if err := createDirIfNotExists(yearDir); err != nil {
		return "", fmt.Errorf("create year directory %s: %v", yearDir, err)
	}

	monthDir := path.Join(yearDir, fmt.Sprintf("%d-%02d", fileCreationYear, fileCreationMonth))
	if err := createDirIfNotExists(monthDir); err != nil {
		return "", fmt.Errorf("create month directory %s: %v", monthDir, err)
	}

	outPath, err := constructOutPath(monthDir, fileName, sortIntoCategories)
	if err != nil {
		return "", fmt.Errorf("construct out path for %s: %v", fileName, err)
	}

	fileContent, err := os.ReadFile(path.Join(sourceDir, fileName))
	if err != nil {
		return "", fmt.Errorf("read file %s: %v", fileName, err)
	}

	if err := os.WriteFile(outPath, fileContent, 0644); err != nil {
		return outPath, fmt.Errorf("write file %s: %v", fileName, err)
	}

	return outPath, nil
}

func constructOutPath(parentPath string, fileName string, sortIntoCategories bool) (string, error) {
	outPath := path.Join(parentPath, fileName)

	if sortIntoCategories {
		categoryDir := outPath

		if isPicture(fileName) {
			categoryDir = path.Join(parentPath, PicturesDirName)
		}

		if isVideo(fileName) {
			categoryDir = path.Join(parentPath, VideosDirName)
		}

		if err := createDirIfNotExists(categoryDir); err != nil {
			return "", fmt.Errorf("create category directory %s: %v", categoryDir, err)
		}

		outPath = path.Join(categoryDir, fileName)
	}

	return outPath, nil
}

func preserveOriginalFileCreationDate(fileInfo os.FileInfo, filePath string) error {
	modifiedTime := fileInfo.ModTime()
	accessTime := fileInfo.ModTime()

	if err := os.Chtimes(filePath, accessTime, modifiedTime); err != nil {
		return fmt.Errorf("set file %s modification time: %v", fileInfo.Name(), err)
	}

	return nil
}

func shouldBeSorted(fileName string, allowedExtensions []string) bool {
	if len(allowedExtensions) == 1 && allowedExtensions[0] == "*" {
		return true
	}

	fileExt := strings.ToLower(filepath.Ext(fileName))

	for _, ext := range allowedExtensions {
		if ext == "*" || ext == fileExt {
			return true
		}
	}
	return false
}

func resolveFileExtensions() []string {
	ext := []string{"*"}

	if fileExtensionsArg != nil && *fileExtensionsArg != "" {
		ext = strings.Split(*fileExtensionsArg, ",")
	}

	for i := 0; i < len(ext); i++ {
		ext[i] = strings.TrimSpace(ext[i])
		ext[i] = strings.ToLower(ext[i])

		if ext[i] == "*" {
			continue
		}

		if !strings.HasPrefix(ext[i], ".") {
			ext[i] = "." + ext[i]
		}
	}

	return ext
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

func isPicture(fileName string) bool {
	normalizedFileName := strings.ToLower(fileName)
	for _, ext := range pictureFileExtensions {
		if strings.HasSuffix(normalizedFileName, ext) {
			return true
		}
	}
	return false
}

func isVideo(fileName string) bool {
	normalizedFileName := strings.ToLower(fileName)
	for _, ext := range videoFileExtensions {
		if strings.HasSuffix(normalizedFileName, ext) {
			return true
		}
	}
	return false
}
