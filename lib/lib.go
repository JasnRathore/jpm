package lib

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
)

// Delete removes a file or directory
func Delete(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to stat %s: %w", path, err)
	}

	if info.IsDir() {
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}

	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", path, err)
	}

	fmt.Printf("✓ Deleted: %s\n", path)
	return nil
}

// Move moves a file or directory from src to dst
func Move(src, dst string) error {
	// Try simple rename first (works if on same filesystem)
	err := os.Rename(src, dst)
	if err == nil {
		fmt.Printf("✓ Moved: %s → %s\n", src, dst)
		return nil
	}

	// If rename fails, copy and delete
	if err := Copy(src, dst); err != nil {
		return err
	}

	if err := Delete(src); err != nil {
		return fmt.Errorf("moved but failed to delete source: %w", err)
	}

	fmt.Printf("✓ Moved: %s → %s\n", src, dst)
	return nil
}

// Copy copies a file or directory from src to dst
func Copy(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if srcInfo.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Copy permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	fmt.Printf("✓ Copied: %s → %s\n", src, dst)
	return nil
}

// MakeExecutable makes a file executable (chmod +x on Unix, no-op on Windows)
func MakeExecutable(path string) error {
	if runtime.GOOS == "windows" {
		return nil // Windows doesn't use Unix permissions
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Add execute permissions for user, group, and others
	newMode := info.Mode() | 0111
	if err := os.Chmod(path, newMode); err != nil {
		return fmt.Errorf("failed to chmod: %w", err)
	}

	fmt.Printf("✓ Made executable: %s\n", path)
	return nil
}

// DetectArchiveType detects the type of archive from filename
func DetectArchiveType(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return "tar.gz"
	case strings.HasSuffix(lower, ".tar"):
		return "tar"
	case strings.HasSuffix(lower, ".zip"):
		return "zip"
	default:
		return "unknown"
	}
}
