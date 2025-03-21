// Package ffmpeg provides functionality for detecting and working with FFmpeg.
// It includes capabilities for detecting FFmpeg installation, version, and support
// for advanced features like QP and CU analysis.
package ffmpeg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Private functions (alphabetical)

// checkCUReadingSupport checks if FFmpeg supports CU value reading capabilities.
// It uses multiple methods to determine support including checking version output,
// looking for debug builds, and testing with a small sample video.
func checkCUReadingSupport(ffmpegPath string, versionOutput string) bool {
	// Method 1: Check configuration flags in version output
	if strings.Contains(versionOutput, "--enable-debug") ||
		strings.Contains(versionOutput, "with-debug") {
		return true
	}

	// Method 2: Check for ffmpeg_g executable (debug build)
	var ffmpegGPath string
	if strings.HasSuffix(ffmpegPath, ".exe") {
		ffmpegGPath = strings.TrimSuffix(ffmpegPath, ".exe") + "_g.exe"
	} else {
		ffmpegGPath = ffmpegPath + "_g"
	}

	if _, err := os.Stat(ffmpegGPath); err == nil {
		return true
	}

	// Method 3: Check for -debug:v cu support
	// Create a temporary file for testing
	tempDir, err := os.MkdirTemp("", "ffmpeg-debug-test")
	if err != nil {
		return false
	}
	defer os.RemoveAll(tempDir)

	// Create a small test file
	testFile := filepath.Join(tempDir, "test.mp4")

	// Try to run FFmpeg with -debug:v cu parameter
	// We'll create a 1-frame test video and check if the debug parameter is accepted
	cmd := exec.Command(
		ffmpegPath,
		"-f", "lavfi",
		"-i", "color=c=black:s=32x32:d=0.1",
		"-debug:v", "cu",
		"-t", "0.1",
		"-y",
		testFile,
	)

	// Capture stderr to check for errors
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return false
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return false
	}

	// Read stderr
	stderrBytes := make([]byte, 4096)
	n, _ := stderr.Read(stderrBytes)
	stderrOutput := string(stderrBytes[:n])

	// Wait for the command to finish
	_ = cmd.Wait()

	// Check if the command was successful and if the debug parameter was accepted
	// If the parameter is not supported, FFmpeg will output an error message
	return !strings.Contains(stderrOutput, "debug:v") &&
		!strings.Contains(stderrOutput, "Unrecognized option") &&
		!strings.Contains(stderrOutput, "Error parsing options")
}

// checkQPReadingSupport checks if FFmpeg supports QP value reading capabilities.
// It uses multiple methods to determine support including checking version output,
// looking for debug builds, and testing with a small sample video.
func checkQPReadingSupport(ffmpegPath string, versionOutput string) bool {
	// Method 1: Check configuration flags in version output
	if strings.Contains(versionOutput, "--enable-debug") ||
		strings.Contains(versionOutput, "with-debug") {
		return true
	}

	// Method 2: Check for ffmpeg_g executable (debug build)
	var ffmpegGPath string
	if strings.HasSuffix(ffmpegPath, ".exe") {
		ffmpegGPath = strings.TrimSuffix(ffmpegPath, ".exe") + "_g.exe"
	} else {
		ffmpegGPath = ffmpegPath + "_g"
	}

	if _, err := os.Stat(ffmpegGPath); err == nil {
		return true
	}

	// Method 3: Check for -debug:v qp support
	// Create a temporary file for testing
	tempDir, err := os.MkdirTemp("", "ffmpeg-debug-test")
	if err != nil {
		return false
	}
	defer os.RemoveAll(tempDir)

	// Create a small test file
	testFile := filepath.Join(tempDir, "test.mp4")

	// Try to run FFmpeg with -debug:v qp parameter
	// We'll create a 1-frame test video and check if the debug parameter is accepted
	cmd := exec.Command(
		ffmpegPath,
		"-f", "lavfi",
		"-i", "color=c=black:s=32x32:d=0.1",
		"-debug:v", "qp",
		"-t", "0.1",
		"-y",
		testFile,
	)

	// Capture stderr to check for errors
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return false
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return false
	}

	// Read stderr
	stderrBytes := make([]byte, 4096)
	n, _ := stderr.Read(stderrBytes)
	stderrOutput := string(stderrBytes[:n])

	// Wait for the command to finish
	_ = cmd.Wait()

	// Check if the command was successful and if the debug parameter was accepted
	// If the parameter is not supported, FFmpeg will output an error message
	return !strings.Contains(stderrOutput, "debug:v") &&
		!strings.Contains(stderrOutput, "Unrecognized option") &&
		!strings.Contains(stderrOutput, "Error parsing options")
}

// extractVersion extracts the version number from FFmpeg version output.
// It parses the output string to find and return just the version number.
func extractVersion(versionOutput string) string {
	// Extract the version from the output
	versionLine := strings.Split(versionOutput, "\n")[0]
	parts := strings.Split(versionLine, " ")
	if len(parts) >= 3 {
		return parts[2]
	}
	return "unknown"
}

// getCommonInstallPaths returns a list of common paths where FFmpeg might be installed
// based on the operating system. It handles Windows, macOS, and Linux systems.
func getCommonInstallPaths(execName string) []string {
	paths := []string{}

	// Check current directory first
	curDir, err := os.Getwd()
	if err == nil {
		paths = append(paths, filepath.Join(curDir, execName))
	}

	// Add OS-specific common install locations
	switch runtime.GOOS {
	case "windows":
		// Windows common paths
		programFiles := os.Getenv("ProgramFiles")
		programFiles86 := os.Getenv("ProgramFiles(x86)")

		if programFiles != "" {
			paths = append(paths, filepath.Join(programFiles, "FFmpeg", "bin", execName))
		}
		if programFiles86 != "" {
			paths = append(paths, filepath.Join(programFiles86, "FFmpeg", "bin", execName))
		}

		// Add user profile directories
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" {
			paths = append(paths, filepath.Join(userProfile, "Downloads", "FFmpeg", "bin", execName))
			paths = append(paths, filepath.Join(userProfile, "Documents", "FFmpeg", "bin", execName))
		}
	case "darwin":
		// macOS common paths
		paths = append(paths, "/usr/local/bin/"+execName)
		paths = append(paths, "/usr/bin/"+execName)
		paths = append(paths, "/opt/homebrew/bin/"+execName)
		paths = append(paths, "/opt/local/bin/"+execName)

		// Add user home directory
		home := os.Getenv("HOME")
		if home != "" {
			paths = append(paths, filepath.Join(home, "bin", execName))
			paths = append(paths, filepath.Join(home, "Applications", "FFmpeg", "bin", execName))
		}
	default:
		// Linux and other Unix-like systems
		paths = append(paths, "/usr/bin/"+execName)
		paths = append(paths, "/usr/local/bin/"+execName)
		paths = append(paths, "/opt/local/bin/"+execName)
		paths = append(paths, "/opt/bin/"+execName)

		// Add user home directory
		home := os.Getenv("HOME")
		if home != "" {
			paths = append(paths, filepath.Join(home, "bin", execName))
			paths = append(paths, filepath.Join(home, "local", "bin", execName))
		}
	}

	return paths
}

// Public functions (alphabetical)

// FindFFmpeg searches for FFmpeg installation on the system and returns information
// about the installation including path, version, and capability support.
func FindFFmpeg() (*FFmpegInfo, error) {
	// First check if FFmpeg is in the PATH
	ffmpegPath, err := exec.LookPath("ffmpeg")

	// If not found in PATH, try common install locations
	if err != nil {
		execName := "ffmpeg"
		if runtime.GOOS == "windows" {
			execName = "ffmpeg.exe"
		}

		// Get common install paths and check each one
		commonPaths := getCommonInstallPaths(execName)
		for _, path := range commonPaths {
			if _, err := os.Stat(path); err == nil {
				ffmpegPath = path
				break
			}
		}

		// If still not found, return error
		if ffmpegPath == "" {
			return &FFmpegInfo{
				Installed: false,
			}, fmt.Errorf("FFmpeg not found in PATH or common install locations")
		}
	}

	// Get FFmpeg version
	cmd := exec.Command(ffmpegPath, "-version")
	versionOutput, err := cmd.Output()
	if err != nil {
		return &FFmpegInfo{
			Installed: true,
			Path:      ffmpegPath,
		}, fmt.Errorf("failed to get FFmpeg version: %v", err)
	}

	// Extract version from output
	version := extractVersion(string(versionOutput))

	// Check if FFmpeg supports QP and CU reading
	supportsQP := checkQPReadingSupport(ffmpegPath, string(versionOutput))
	supportsCU := checkCUReadingSupport(ffmpegPath, string(versionOutput))

	return &FFmpegInfo{
		Installed:               true,
		Path:                    ffmpegPath,
		Version:                 version,
		HasQPReadingInfoSupport: supportsQP,
		HasCUReadingInfoSupport: supportsCU,
	}, nil
}
