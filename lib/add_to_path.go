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

func AddToPath(dir string) (string, error) {
	pmPath, err := getSelfPath()
	if err != nil {
		return "", err
	}
	fullPath := filepath.Join(pmPath, dir)

	switch runtime.GOOS {
	case "windows":
		// Get user-only PATH (not merged with system PATH)
		getCmd := exec.Command("powershell", `[Environment]::GetEnvironmentVariable('Path', 'User')`)
		out, err := getCmd.Output()
		if err != nil {
			return "", err
		}
		currentPath := strings.TrimSpace(string(out))

		// Split and check for duplicates (case-insensitive)
		paths := strings.Split(currentPath, ";")
		for _, p := range paths {
			if strings.EqualFold(strings.TrimSpace(p), dir) {
				return dir, nil // Already exists
			}
		}

		// Append safely and update user PATH
		newPath := currentPath
		if currentPath != "" {
			newPath += ";"
		}
		newPath += dir

		// Escape double quotes for PowerShell command
		psCmd := fmt.Sprintf(`[Environment]::SetEnvironmentVariable('Path', "%s", 'User')`, strings.ReplaceAll(newPath, `"`, `\"`))
		cmd := exec.Command("powershell", psCmd)
		return dir, cmd.Run()

	case "linux", "darwin":
		usr, err := user.Current()
		if err != nil {
			return fullPath, err
		}

		rcFile := filepath.Join(usr.HomeDir, ".bashrc")
		if _, err := os.Stat(filepath.Join(usr.HomeDir, ".zshrc")); err == nil {
			rcFile = filepath.Join(usr.HomeDir, ".zshrc")
		}

		line := fmt.Sprintf("\nexport PATH=\"$PATH:%s\"\n", fullPath)
		f, err := os.OpenFile(rcFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return fullPath, err
		}
		defer f.Close()

		_, err = f.WriteString(line)
		return fullPath, err

	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
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
		// Fetch current user PATH, filter out target dir, and save
		psScript := fmt.Sprintf(`
			$path = [Environment]::GetEnvironmentVariable('Path', 'User')
			$new = ($path -split ';' | Where-Object { $_ -and ($_ -ne '%s') }) -join ';'
			[Environment]::SetEnvironmentVariable('Path', $new, 'User')
		`, fullPath)
		cmd := exec.Command("powershell", psScript)
		return cmd.Run()

	case "linux", "darwin":
		usr, err := user.Current()
		if err != nil {
			return err
		}
		rcFile := filepath.Join(usr.HomeDir, ".bashrc")
		if _, err := os.Stat(filepath.Join(usr.HomeDir, ".zshrc")); err == nil {
			rcFile = filepath.Join(usr.HomeDir, ".zshrc")
		}

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
		return os.WriteFile(rcFile, []byte(strings.Join(newLines, "\n")), 0644)

	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}
