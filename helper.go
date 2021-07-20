package main

import (
	"fmt"
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
			_, e := CopyFileLazyInfo(path, myDest, info)
			return e
		}
	})

	return finalErr
}

func CopyFileLazy(src string, dest string) (int64, error) {
	sInfo, e := os.Stat(src)
	if e != nil {
		return 0, e
	}

	return CopyFileLazyInfo(src, dest, sInfo)

}

func CopyFileLazyInfo(src string, dest string, info os.FileInfo) (int64, error) {

	dInfo, e := os.Stat(dest)
	if e != nil && dInfo != nil {
		if (dInfo.ModTime() == info.ModTime()) && (dInfo.Size() == info.Size()) {
			return dInfo.Size(), nil
		}
	}

	i, e := os.Open(src)
	if e != nil {
		log.Println("Error: Open " + src)
		return 0, e
	}
	defer i.Close()
	o, e := os.Create(dest)
	if e != nil {
		log.Println("Error: Close " + dest)
		return 0, e
	}
	defer o.Close()

	size, e := o.ReadFrom(i)
	os.Chtimes(dest, info.ModTime(), info.ModTime())

	if e != nil {
		return 0, fmt.Errorf("%s | CopyFileLazyInfo - \n\t[%s] -> \n\t[%s]", e.Error(), src, dest)
	}

	return size, e
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
