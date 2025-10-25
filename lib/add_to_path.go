package lib

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

func getSelfPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exePath)
	return filepath.Join(dir, "bin"), nil
}

func AddToPath(dir string) error {
	pmPath, err := getSelfPath()
	if err != nil {

	}
	fullPath := filepath.Join(pmPath, dir)
	switch runtime.GOOS {
	case "windows":
		// Update user PATH (persistent, no admin)
		cmd := exec.Command("powershell",
			fmt.Sprintf(`[Environment]::SetEnvironmentVariable('Path', $env:Path + ';%s', 'User')`, fullPath))
		return cmd.Run()

	case "linux", "darwin":
		usr, err := user.Current()
		if err != nil {
			return err
		}

		rcFile := filepath.Join(usr.HomeDir, ".bashrc")
		// For zsh users, prefer .zshrc if it exists
		if _, err := os.Stat(filepath.Join(usr.HomeDir, ".zshrc")); err == nil {
			rcFile = filepath.Join(usr.HomeDir, ".zshrc")
		}

		line := fmt.Sprintf("\nexport PATH=\"$PATH:%s\"\n", fullPath)
		f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = f.WriteString(line)
		return err

	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

func RemoveFromPath(dir string) error {
	pmPath, err := getSelfPath()
	if err != nil {
		return err
	}
	fullPath := filepath.Join(pmPath, dir)
	switch runtime.GOOS {
	case "windows":
		// Remove from user PATH (persistent, no admin)
		cmd := exec.Command("powershell",
			fmt.Sprintf(`[Environment]::SetEnvironmentVariable('Path', ($env:Path -split ';' | Where-Object { $_ -ne '%s' }) -join ';', 'User')`, fullPath))
		return cmd.Run()

	case "linux", "darwin":
		usr, err := user.Current()
		if err != nil {
			return err
		}
		rcFile := filepath.Join(usr.HomeDir, ".bashrc")
		// For zsh users, prefer .zshrc if it exists
		if _, err := os.Stat(filepath.Join(usr.HomeDir, ".zshrc")); err == nil {
			rcFile = filepath.Join(usr.HomeDir, ".zshrc")
		}

		// Read file contents
		fileBytes, err := os.ReadFile(rcFile)
		if err != nil {
			return err
		}
		lines := strings.Split(string(fileBytes), "\n")
		var newLines []string
		for _, line := range lines {
			if !strings.Contains(line, fullPath) {
				newLines = append(newLines, line)
			}
		}
		// Overwrite rcFile
		return os.WriteFile(rcFile, []byte(strings.Join(newLines, "\n")), 0644)

	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}
