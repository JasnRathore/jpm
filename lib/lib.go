package lib

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
)

func Extract(filepath string) error {
	return nil
}
func Delete(filepath string) error {
	return os.Remove(filepath)
}
func ExtractZip(src, dest string) (string, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return "", nil // prevent ZipSlip
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, f.Mode())
		} else {
			if err = os.MkdirAll(filepath.Dir(fpath), f.Mode()); err != nil {
				return "", err
			}
			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return "", err
			}
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			_, err = io.Copy(outFile, rc)
			outFile.Close()
			rc.Close()
			if err != nil {
				return "", err
			}
		}
	}
	fmt.Printf("Extracted: %s at %s\n", src, dest)
	return dest, nil
}
