package main

import (
	"io"
	"os"
	"path/filepath"
)

// Copy a directory tree from `src` to `dest`
func CopyTree(src string, dest string, verbose bool) error {
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

			if (err != nil) && (os.IsExist(err) == false) {
				return err
			}

			return nil

		} else {
			var destFile, srcFile *os.File

			srcFile, err = os.Open(path)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			destFile, err = os.OpenFile(myDest, os.O_WRONLY|os.O_CREATE, info.Mode())
			if err != nil {
				return err
			}

			_, err = io.CopyN(destFile, srcFile, info.Size())
			destFile.Close()
			os.Chtimes(myDest, info.ModTime(), info.ModTime())

			if err != nil {
				return err
			}

			return nil
		}
	})

	return finalErr
}
