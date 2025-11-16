package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/rwcarlsen/goexif/exif"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	PicturesDirName = "pictures"
	VideosDirName   = "videos"
	GifsDirName     = "gifs"
)

var pictureFileExtensions = []string{".jpg", ".png", ".heic", ".jpeg", ".dng", ".arw"}
var videoFileExtensions = []string{".mp4", ".mov", ".webp"}
var gifFileExtensions = []string{".gif"}
var fileDateTimeFormats = []string{
	"2006-01-02_15-04-05",
	"2006-01-02",
	"20060102",
	"20060102_150405",
	"20060102_150405",
	"PXL_20060102_150405",
}

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

	fileChan := make(chan fs.DirEntry)
	var wg sync.WaitGroup

	// Start worker goroutines for each CPU core
	numWorkers := runtime.NumCPU()
	log.Printf("starting %d workers", numWorkers)

	for workerIndex := 0; workerIndex < numWorkers; workerIndex++ {
		wg.Add(1)

		go func(worker int) {
			defer wg.Done()

			// Process files from the channel
			for item := range fileChan {
				if item.IsDir() {
					continue
				}

				fileName := item.Name()
				fileInfo, err := item.Info()
				if err != nil {
					log.Printf("%d/%d: ERROR: get file info for %s: %v", worker, numWorkers, fileName, err)
					continue
				}

				if !shouldBeSorted(fileName, fileExtensions) {
					log.Printf("%d/%d: file %s does not match allowed file extensions %+v, skipping", worker, numWorkers, fileName, fileExtensions)
					continue
				}

				log.Printf("%d/%d: copying file %s", worker, numWorkers, fileName)
				outPath, err := copyFile(fileInfo, sourceDir, outDir, sortIntoCategories)
				if err != nil {
					log.Printf("%d/%d: ERROR: copy file %s: %v", worker, numWorkers, fileName, err)
					continue
				}

				if err := preserveOriginalFileCreationDate(fileInfo, outPath); err != nil {
					log.Printf("%d/%d: ERROR: preserve original file creation date: %v", worker, numWorkers, err)
				}

				log.Printf("%d/%d: file %s copied to %s", worker, numWorkers, fileName, outPath)
			}
		}(workerIndex)
	}

	// Send files to be processed by each worker
	for _, item := range items {
		fileChan <- item
	}
	close(fileChan)

	// Wait for all workers to finish
	wg.Wait()

	return nil
}

func copyFile(fileInfo fs.FileInfo, sourceDir string, outDir string, sortIntoCategories bool) (string, error) {
	fileName := fileInfo.Name()
	fileOutDir := ""

	// Get the date when the file was created (ideally when the picture was taken)
	fileCreationDate, err := getFileCreatedDateTime(fileInfo, sourceDir)
	if fileCreationDate == nil || err != nil {
		unknownDirName := "_UnknownDateTime"

		log.Printf("file %s creation date unknown, putting into %s", fileName, unknownDirName)

		unknownDir := path.Join(outDir, unknownDirName)
		if err := createDirIfNotExists(unknownDir); err != nil {
			return "", fmt.Errorf("create unknown date time directory %s: %v", unknownDir, err)
		}
		fileOutDir = unknownDir
	} else {
		// Use the year and month to sort the files into subdirectories
		fileCreationYear := fileCreationDate.Year()
		fileCreationMonth := fileCreationDate.Month()
		fileCreationDay := fileCreationDate.Day()

		log.Printf("file %s created on %d-%02d-%02d", fileName, fileCreationYear, fileCreationMonth, fileCreationDay)

		// Put files into subdirectories on the format YYYY-MM
		monthDir := path.Join(outDir, fmt.Sprintf("%d-%02d", fileCreationYear, fileCreationMonth))
		if err := createDirIfNotExists(monthDir); err != nil {
			return "", fmt.Errorf("create month directory %s: %v", monthDir, err)
		}
		fileOutDir = monthDir
	}

	outPath, err := constructOutPath(fileOutDir, fileName, sortIntoCategories)
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

func getFileCreatedDateTime(fileInfo fs.FileInfo, fileDir string) (*time.Time, error) {

	filePath := path.Join(fileDir, fileInfo.Name())

	// First try to get the date taken from the EXIF data
	dateTaken, err := getExifDateTaken(filePath)
	if err == nil && dateTaken != nil {
		// Ignore the error and return the date taken if it was successfully retrieved
		return dateTaken, nil
	}

	// If the EXIF data is not available, try to get the date taken from the file name
	dateTaken, err = getDateTakenFromFileName(fileInfo.Name())
	if err == nil && dateTaken != nil {
		return dateTaken, nil
	}

	dateTaken, err = getDateTakenFromFileMediaCreatedAttribute(filePath)
	if err == nil && dateTaken != nil {
		return dateTaken, nil
	}

	if dateTaken == nil {
		return nil, fmt.Errorf("unable to determine date taken for file %s", filePath)
	}

	return dateTaken, nil

	//// If we can't get the date from EXIF or the file name, fall back to get the file's modified time on disk.
	//// This will most likely be the datetime for when the file was copied to this hard drive instead of when the picture was actually taken (unfortunately).
	//created := fileInfo.ModTime()
	//
	//if runtime.GOOS == "windows" {
	//	// On Windows, we can get the file creation time from the file attributes
	//	attr := fileInfo.Sys().(*syscall.Win32FileAttributeData)
	//	created = time.Unix(0, attr.CreationTime.Nanoseconds())
	//}
	//
	//return created
}

// On windows, media files may have a "Media Created" attribute that can be used to get the date the media was created.
// How to inspect a file: On a file, right click -> Properties -> Details -> Origin -> Media created
func getDateTakenFromFileMediaCreatedAttribute(path string) (*time.Time, error) {
	abs, _ := filepath.Abs(path)
	abs = strings.ReplaceAll(abs, "/", `\`)

	ole.CoInitialize(0)
	defer ole.CoUninitialize()

	// Create Shell object
	shellObj, err := oleutil.CreateObject("Shell.Application")
	if err != nil {
		return nil, err
	}
	shell, err := shellObj.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, err
	}
	defer shell.Release()

	// Split into folder + file name
	folderPath, fileName := filepath.Split(abs)

	folderObj, err := oleutil.CallMethod(shell, "NameSpace", folderPath)
	if err != nil || folderObj == nil {
		return nil, fmt.Errorf("failed to open folder")
	}
	folder := folderObj.ToIDispatch()
	defer folder.Release()

	fileObj, err := oleutil.CallMethod(folder, "ParseName", fileName)
	if err != nil || fileObj == nil {
		return nil, fmt.Errorf("failed to resolve file in folder")
	}
	item := fileObj.ToIDispatch()
	defer item.Release()

	// See ./cmd/debug.main.go for listing all possible property indexes
	const mediaCreatedIndex = 208 // 208 = "Media created" in Windows property index map
	const dateTakenIndex = 12     // 12 = "Date taken" in Windows property index map

	mediaCreatedObj, err := oleutil.CallMethod(folder, "GetDetailsOf", item, mediaCreatedIndex)
	if err != nil || mediaCreatedObj == nil {
		mediaCreatedObj, err = oleutil.CallMethod(folder, "GetDetailsOf", item, dateTakenIndex)
		if err != nil || mediaCreatedObj == nil {
			return nil, fmt.Errorf("property lookup failed")
		}
	}

	raw := strings.TrimSpace(mediaCreatedObj.ToString())
	if raw == "" {
		return nil, fmt.Errorf("no value in Media Created field")
	}

	// Windows formats: "dd.MM.yyyy HH:mm"
	parsed, err := time.Parse("1/2/2006 3:04 PM", raw)
	if err != nil {
		// Try another common Windows format
		parsed, err = time.Parse("2006-01-02 15:04:05", raw)
		if err != nil {
			return nil, fmt.Errorf("parse error: %w (%s)", err, raw)
		}
	}

	return &parsed, nil
}

func getDateTakenFromFileName(fileName string) (*time.Time, error) {
	// Getting the date the picture was taken from the file name is a hail mary since if the camera follows a date format, it most likely also writes the date to the EXIF data.
	// But in a rare case where we couldn't get the EXIF data, we can try to parse the date from the file name as a fallback.
	for _, format := range fileDateTimeFormats {
		dateTaken, err := time.Parse(format, fileName)
		if err == nil {
			log.Printf("parsed date taken %s from file name %s", dateTaken, fileName)
			return &dateTaken, nil
		}
	}

	return nil, fmt.Errorf("no date taken found in file name")
}

func getExifDateTaken(filePath string) (*time.Time, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file %s: %v", filePath, err)
	}
	defer file.Close()

	x, err := exif.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode exif data from file %s: %v", filePath, err)
	}

	dateTaken, err := x.DateTime()
	if err != nil {
		return nil, fmt.Errorf("get Date Taken from exif data: %v", err)
	}

	return &dateTaken, nil
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

		if isGif(fileName) {
			categoryDir = path.Join(parentPath, GifsDirName)
		}

		if err := createDirIfNotExists(categoryDir); err != nil {
			return "", fmt.Errorf("create category directory %s: %v", categoryDir, err)
		}

		outPath = path.Join(categoryDir, fileName)
	}

	return outPath, nil
}

func preserveOriginalFileCreationDate(fileInfo os.FileInfo, filePath string) error {
	dirPath := path.Dir(filePath)

	createdTime, err := getFileCreatedDateTime(fileInfo, dirPath)
	if createdTime == nil {
		return fmt.Errorf("get original file creation date for %s: %v", fileInfo.Name(), err)
	}

	if runtime.GOOS == "windows" {
		return setWindowsFileCreationDateTime(filePath, *createdTime)
	}

	modifiedTime := *createdTime
	accessTime := *createdTime

	if err := os.Chtimes(filePath, accessTime, modifiedTime); err != nil {
		return fmt.Errorf("set file %s modification time: %v", fileInfo.Name(), err)
	}

	return nil
}

// setWindowsFileCreationDateTime sets the creation time of a file on Windows using Windows APIs via syscall.
func setWindowsFileCreationDateTime(filename string, ctime time.Time) error {
	// Convert the filename to a UTF16 pointer
	filePath, err := syscall.UTF16PtrFromString(filename)
	if err != nil {
		return fmt.Errorf("resolve filePath from filename %s: %v", filename, err)
	}

	// Open the file with proper permissions to modify the file times
	handle, err := syscall.CreateFile(
		filePath,
		syscall.FILE_WRITE_ATTRIBUTES, syscall.FILE_SHARE_WRITE, nil,
		syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)

	if err != nil {
		return fmt.Errorf("open file %v: %v", *filePath, err)
	}
	defer func() {
		if err := syscall.CloseHandle(handle); err != nil {
			log.Fatalf("close syscall filehandler for %s: %v", filename, err)
		}
	}()

	// Create a Filetime structure from the Go time
	fileTime := syscall.NsecToFiletime(ctime.UnixNano())

	// Set the creation time (leaving access and write times as nil will not modify them)
	err = syscall.SetFileTime(handle, &fileTime, nil, nil)
	if err != nil {
		return fmt.Errorf("update file time for %s to %+v: %v", filename, fileTime, err)
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

func isGif(fileName string) bool {
	normalizedFileName := strings.ToLower(fileName)
	for _, ext := range gifFileExtensions {
		if strings.HasSuffix(normalizedFileName, ext) {
			return true
		}
	}
	return false
}
