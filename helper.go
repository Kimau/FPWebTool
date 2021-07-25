package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
)

// Copy a directory tree from `src` to `dest`
func CopyTree(src string, dest string, verbose bool) error {
	src = filepath.Clean(src)
	dest = filepath.Clean(dest)

	finalErr := filepath.Walk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		myDest, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		myDest = filepath.Join(dest, myDest)

		if info.IsDir() {
			err = os.Mkdir(myDest, info.Mode())

			if (err != nil) && (!os.IsExist(err)) {
				return err
			}

			return nil

		} else {
			_, e := CopyFileLazy(path, myDest)
			return e
		}
	})

	return finalErr
}

func CopyFileLazy(src string, dest string) (int64, error) {

	srcInfo, err := os.Stat(src)
	if err != nil {
		fmt.Println("Couldn't Read:" + src)
		return 0, err
	}

	destInfo, err := os.Stat(dest)
	if err != nil && destInfo != nil {
		if (destInfo.ModTime() == srcInfo.ModTime()) && (destInfo.Size() == srcInfo.Size()) {
			return destInfo.Size(), nil
		}
	}

	source, err := os.Open(src)
	if err != nil {
		fmt.Println("Error Reading:" + src)
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dest)
	if err != nil {
		fmt.Println("Error Writing:" + src)
		return 0, err
	}
	defer destination.Close()

	nBytes, err := io.Copy(destination, source)
	if err != nil {
		fmt.Println("Error Copying:" + src + ">" + dest)
		return 0, err
	}
	if nBytes != srcInfo.Size() {
		return 0, fmt.Errorf("failed to copy %d != %d", nBytes, srcInfo.Size())
	}
	return nBytes, err

}

func CheckErr(err error) {
	if err != nil {
		log.Fatalf(`
---- STACK -------------
%s
----  END  -------------

%s`, debug.Stack(), err.Error())
	}
}

func CheckErrContext(err error, context ...string) {
	if err != nil {
		log.Fatalf(`
---- STACK -------------
%s
----  END  -------------

%s
%s`, debug.Stack(), err.Error(), context)
	}
}
