package lib

import (
	"errors"
	"fmt"
	"github.com/dustin/go-humanize"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
)

type WriteCounter struct {
	Total uint64
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	wc.PrintProgress()
	return n, nil
}

// PrintProgress prints the progress of a file write
func (wc WriteCounter) PrintProgress() {
	// Clear the line by using a character return to go back to the start and remove
	// the remaining characters by filling it with spaces
	fmt.Printf("\r%s", strings.Repeat(" ", 50))

	// Return again and print current status of download
	// We use the humanize package to print the bytes in a meaningful way (e.g. 10 MB)
	fmt.Printf("\rDownloading... %s complete", humanize.Bytes(wc.Total))
}

func Download(rawURL string, dir string) error {
	// Make request
	resp, err := http.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Try to get filename from URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	filename := path.Base(parsedURL.Path)

	// If URL doesn't give us a filename, try to get it from response header
	if filename == "" || filename == "/" {
		contentDisp := resp.Header.Get("Content-Disposition")
		re := regexp.MustCompile(`(?i)filename="?([^"]+)"?`)
		if matches := re.FindStringSubmatch(contentDisp); len(matches) > 1 {
			filename = matches[1]
		} else {
			// Fallback default name
			filename = "downloaded_file"
		}
	}

	// Full path
	fullPath := path.Join(dir, filename)
	fmt.Println(fullPath)
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(dir, os.ModePerm)
		if err != nil {
			fmt.Println(err)
		}
	}
	// Create temp file
	out, err := os.Create(fullPath + ".tmp")
	if err != nil {
		return err
	}
	defer out.Close()

	// Copy with counter
	counter := &WriteCounter{}
	_, err = io.Copy(out, io.TeeReader(resp.Body, counter))
	if err != nil {
		return err
	}

	fmt.Println() // newline after progress
	out.Close()

	// Rename tmp → actual file
	if err := os.Rename(fullPath+".tmp", fullPath); err != nil {
		return err
	}

	fmt.Println("Downloaded:", fullPath)
	return nil
}
