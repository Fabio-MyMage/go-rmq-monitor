package pidfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// PIDFile represents a PID file lock
type PIDFile struct {
	path string
}

// New creates a new PID file at the specified path
func New(path string) *PIDFile {
	return &PIDFile{path: path}
}

// Create creates and locks the PID file
// Returns an error if another instance is already running
func (p *PIDFile) Create() error {
	// Check if PID file already exists
	if _, err := os.Stat(p.path); err == nil {
		// File exists, check if process is still running
		data, err := os.ReadFile(p.path)
		if err != nil {
			return fmt.Errorf("failed to read existing PID file: %w", err)
		}

		pidStr := strings.TrimSpace(string(data))
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			// Invalid PID file, remove it
			os.Remove(p.path)
		} else {
			// Check if process is still running
			if isProcessRunning(pid) {
				return fmt.Errorf("another instance is already running (PID: %d)", pid)
			}
			// Process is dead, remove stale PID file
			os.Remove(p.path)
		}
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(p.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		// If we can't create the directory, try /tmp as fallback
		if strings.HasPrefix(p.path, "/var/run/") {
			p.path = "/tmp/" + filepath.Base(p.path)
			return p.Create() // Retry with /tmp path
		}
		return fmt.Errorf("failed to create PID file directory: %w", err)
	}

	// Write current PID to file
	pid := os.Getpid()
	if err := os.WriteFile(p.path, []byte(fmt.Sprintf("%d\n", pid)), 0644); err != nil {
		// If we can't write to /var/run, try /tmp as fallback
		if strings.HasPrefix(p.path, "/var/run/") {
			p.path = "/tmp/" + filepath.Base(p.path)
			return p.Create() // Retry with /tmp path
		}
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// Remove removes the PID file
func (p *PIDFile) Remove() error {
	if err := os.Remove(p.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}
	return nil
}

// isProcessRunning checks if a process with the given PID is running
func isProcessRunning(pid int) bool {
	// Send signal 0 to check if process exists
	// This doesn't actually send a signal, just checks if we can
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix systems, FindProcess always succeeds, so we need to check if signal works
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return false
	}

	return true
}

// GetDefaultPath returns the default PID file path based on config file location
func GetDefaultPath(configPath string) string {
	// If config path is absolute, use the same directory
	if filepath.IsAbs(configPath) {
		dir := filepath.Dir(configPath)
		return filepath.Join(dir, "go-rmq-monitor.pid")
	}
	
	// Try /var/run first (standard location for daemon PID files)
	varRunPath := "/var/run/go-rmq-monitor.pid"
	if isWritable("/var/run") {
		return varRunPath
	}
	
	// Fall back to /tmp if /var/run is not writable
	return "/tmp/go-rmq-monitor.pid"
}

// isWritable checks if a directory is writable
func isWritable(path string) bool {
	// Try to create a temporary file to test write permissions
	testFile := filepath.Join(path, ".go-rmq-monitor-test")
	f, err := os.OpenFile(testFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testFile)
	return true
}
