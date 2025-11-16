//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: inspect-media-meta <file>")
		os.Exit(1)
	}

	path, _ := filepath.Abs(os.Args[1])
	path = strings.ReplaceAll(path, "/", `\`)

	ole.CoInitialize(0)
	defer ole.CoUninitialize()

	// Shell object
	shellObj, err := oleutil.CreateObject("Shell.Application")
	if err != nil {
		panic(err)
	}
	shell, err := shellObj.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		panic(err)
	}
	defer shell.Release()

	folderPath, fileName := filepath.Split(path)

	folderObj, err := oleutil.CallMethod(shell, "NameSpace", folderPath)
	if err != nil || folderObj == nil {
		panic("Failed to open folder")
	}
	folder := folderObj.ToIDispatch()
	defer folder.Release()

	fileObj, err := oleutil.CallMethod(folder, "ParseName", fileName)
	if err != nil || fileObj == nil {
		panic("File not found in folder")
	}
	item := fileObj.ToIDispatch()
	defer item.Release()

	fmt.Printf("Inspecting: %s\n\n", filepath.Base(path))
	fmt.Println("Index | Property Name | Value")
	fmt.Println(strings.Repeat("-", 80))

	for i := 0; i < 400; i++ {
		nameObj, _ := oleutil.CallMethod(folder, "GetDetailsOf", nil, i)
		if nameObj == nil {
			continue
		}

		name := strings.TrimSpace(nameObj.ToString())
		if name == "" {
			continue
		}

		valueObj, _ := oleutil.CallMethod(folder, "GetDetailsOf", item, i)
		var value string
		if valueObj != nil {
			value = strings.TrimSpace(valueObj.ToString())
		}

		if value != "" {
			fmt.Printf("%3d  | %-30s | %s\n", i, name, value)
		}
	}
}
