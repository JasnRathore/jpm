package lib

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractZip extracts a zip archive to the destination directory
func ExtractZip(src, dest string) (string, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return "", fmt.Errorf("failed to open zip: %w", err)
	}
	defer r.Close()

	// Ensure destination exists
	if err := os.MkdirAll(dest, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination: %w", err)
	}

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)

		// Security check: prevent ZipSlip
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return "", fmt.Errorf("illegal file path in zip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, f.Mode()); err != nil {
				return "", err
			}
			continue
		}

		// Create parent directories
		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return "", err
		}

		// Extract file
		if err := extractZipFile(f, fpath); err != nil {
			return "", fmt.Errorf("failed to extract %s: %w", f.Name, err)
		}
	}

	fmt.Printf("✓ Extracted: %s → %s\n", src, dest)
	return dest, nil
}

func extractZipFile(f *zip.File, fpath string) error {
	outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = io.Copy(outFile, rc)
	return err
}

// ExtractTar extracts a tar archive to the destination directory
func ExtractTar(src, dest string) (string, error) {
	file, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("failed to open tar: %w", err)
	}
	defer file.Close()

	return extractTarReader(tar.NewReader(file), dest, src)
}

// ExtractTarGz extracts a tar.gz archive to the destination directory
func ExtractTarGz(src, dest string) (string, error) {
	file, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("failed to open tar.gz: %w", err)
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	return extractTarReader(tar.NewReader(gzr), dest, src)
}

func extractTarReader(tr *tar.Reader, dest, src string) (string, error) {
	// Ensure destination exists
	if err := os.MkdirAll(dest, 0755); err != nil {
		return "", fmt.Errorf("failed to create destination: %w", err)
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("tar read error: %w", err)
		}

		target := filepath.Join(dest, header.Name)

		// Security check: prevent path traversal
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return "", fmt.Errorf("illegal file path in tar: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := extractTarFile(tr, target, header); err != nil {
				return "", fmt.Errorf("failed to extract %s: %w", header.Name, err)
			}
		case tar.TypeSymlink:
			if err := os.Symlink(header.Linkname, target); err != nil {
				return "", fmt.Errorf("failed to create symlink %s: %w", header.Name, err)
			}
		default:
			fmt.Printf("Warning: skipping unsupported type %c for %s\n", header.Typeflag, header.Name)
		}
	}

	fmt.Printf("✓ Extracted: %s → %s\n", src, dest)
	return dest, nil
}

func extractTarFile(tr *tar.Reader, target string, header *tar.Header) error {
	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}

	outFile, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, tr)
	return err
}
