package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: winzip source/dir output.zip")
		os.Exit(1)
	}

	output, err := os.Create(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer output.Close()
	zipWriter := zip.NewWriter(output)
	defer zipWriter.Close()

	err = filepath.Walk(os.Args[1], func(path string, info os.FileInfo, err error) error {
		if err != nil || os.Args[1] == path || info.IsDir() {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Get the relative path, and convert separators to /.
		relPath, err := filepath.Rel(os.Args[1], path)
		if err != nil {
			return err
		}
		relPath = strings.Replace(relPath, string(filepath.Separator), "/", -1)
		header.Name = relPath

		partWriter, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		source, err := os.Open(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(partWriter, source)
		if err != nil {
			return err
		}

		return source.Close()
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
