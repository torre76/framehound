// Package ffmpeg provides functionality for detecting and working with FFmpeg.
// It includes capabilities for detecting FFmpeg installation, version, and support
// for advanced features like QP and CU analysis.
package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Private functions (alphabetical)

// checkCUReadingSupport determines if the installed FFmpeg version supports coding unit (CU) analysis.
// It employs multiple detection methods including checking for debug flags in the build configuration,
// looking for debug builds (ffmpeg_g), and attempting a test command with debug options.
// This capability is essential for detailed encoding analysis of HEVC/H.265 and AV1 videos.
func checkCUReadingSupport(ffmpegPath string, versionOutput string) bool {
	// Method 1: Check configuration flags in version output
	if strings.Contains(versionOutput, "--enable-debug") ||
		strings.Contains(versionOutput, "with-debug") {
		return true
	}

	// Method 2: Check for ffmpeg_g executable (debug build)
	debugPath := ""
	if strings.HasSuffix(ffmpegPath, ".exe") {
		debugPath = strings.TrimSuffix(ffmpegPath, ".exe") + "_g.exe"
	} else {
		debugPath = ffmpegPath + "_g"
	}

	if _, err := os.Stat(debugPath); err == nil {
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
	cmd := exec.Command(ffmpegPath, "-hide_banner", "-v", "error", "-debug:v", "cu", "-f", "lavfi", "-i", "testsrc=duration=1:size=192x108:rate=1", "-c:v", "libx265", "-f", "null", testFile)
	if err := cmd.Run(); err == nil {
		return true
	}

	return false
}

// checkFFmpegExistence confirms if FFmpeg is installed on the system by searching for the executable.
// It first looks for the ffmpeg executable in the user's PATH environment variable.
// If not found there, it checks common installation directories based on the operating system.
// This is the foundational check before attempting any FFmpeg operations.
func checkFFmpegExistence() (string, bool) {
	// Try to find FFmpeg in PATH
	pathCmd, err := exec.LookPath("ffmpeg")
	if err == nil {
		return pathCmd, true
	}

	// Get common paths and check each one
	searchPaths := getCommonInstallPaths()
	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}

	// Final check for Windows: Look for FFmpeg in any directories added to PATHEXT
	if runtime.GOOS == "windows" {
		pathExt := os.Getenv("PATHEXT")
		pathExtDirs := strings.Split(pathExt, ";")
		for _, dir := range pathExtDirs {
			dir = strings.TrimSpace(dir)
			if dir == "" {
				continue
			}
			ffmpegPath := filepath.Join(dir, "ffmpeg.exe")
			if _, err := os.Stat(ffmpegPath); err == nil {
				return ffmpegPath, true
			}
		}
	}

	return "", false
}

// checkQPReadingSupport evaluates if the FFmpeg installation supports quantization parameter analysis.
// It checks for debug capabilities in the build, presence of debug versions, and tests
// a small command with QP debug flags. This functionality is crucial for analyzing
// video encoding quality metrics across H.264, HEVC, VP9, and AV1 codecs.
func checkQPReadingSupport(ffmpegPath string, versionOutput string) bool {
	// Method 1: Check configuration flags in version output
	if strings.Contains(versionOutput, "--enable-debug") ||
		strings.Contains(versionOutput, "with-debug") {
		return true
	}

	// Method 2: Check for ffmpeg_g executable (debug build)
	debugPath := ""
	if strings.HasSuffix(ffmpegPath, ".exe") {
		debugPath = strings.TrimSuffix(ffmpegPath, ".exe") + "_g.exe"
	} else {
		debugPath = ffmpegPath + "_g"
	}

	if _, err := os.Stat(debugPath); err == nil {
		return true
	}

	// Method 3: Check for -debug:v qp support
	// Create a temporary file for testing
	tempDir, err := os.MkdirTemp("", "ffmpeg-debug-test")
	if err != nil {
		return false
	}
	defer os.RemoveAll(tempDir)

	// Try to run FFmpeg with -debug:v qp parameter
	cmd := exec.Command(ffmpegPath, "-hide_banner", "-v", "error", "-debug:v", "qp", "-f", "lavfi", "-i", "testsrc=duration=1:size=192x108:rate=1", "-c:v", "libx264", "-f", "null", "-")
	if err := cmd.Run(); err == nil {
		return true
	}

	return false
}

// extractVersionInfo parses the FFmpeg version output to extract detailed version information.
// It identifies the FFmpeg version number, build configuration, and supported libraries
// by analyzing the text output from the ffmpeg -version command. This information
// helps determine feature compatibility and capabilities of the installed FFmpeg.
func extractVersionInfo(versionOutput string) (string, string, []string) {
	lines := strings.Split(versionOutput, "\n")
	if len(lines) == 0 {
		return "", "", nil
	}

	// Extract version from first line
	version := ""
	if len(lines) > 0 {
		firstLine := lines[0]
		versionParts := strings.Split(firstLine, " version ")
		if len(versionParts) > 1 {
			remainingParts := strings.Split(versionParts[1], " ")
			if len(remainingParts) > 0 {
				version = remainingParts[0]
			}
		}

		// If no version found but contains (c), extract that instead
		if version == "" && strings.Contains(firstLine, "(c)") {
			version = "(c)"
		}
	}

	// Extract configuration
	configuration := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "  configuration:") {
			configuration = strings.TrimSpace(strings.TrimPrefix(line, "  configuration:"))
			break
		}
	}

	// Extract libraries
	var libraries []string
	capturingLibraries := false
	for _, line := range lines {
		if strings.HasPrefix(line, "  libavutil") || strings.HasPrefix(line, "  lib") {
			capturingLibraries = true
		}

		if capturingLibraries && strings.HasPrefix(line, "  lib") {
			libraries = append(libraries, strings.TrimSpace(line))
		}

		// Stop when we hit a line that doesn't start with "  lib" after we've started capturing
		if capturingLibraries && !strings.HasPrefix(line, "  lib") && line != "" {
			capturingLibraries = false
		}
	}

	return version, configuration, libraries
}

// getFFmpegVersion retrieves and parses the version information from the FFmpeg executable.
// It executes the ffmpeg -version command, then extracts the version number,
// build configuration options, and linked libraries from the output.
// This information is essential for determining feature compatibility.
func getFFmpegVersion(ffmpegPath string) (string, string, []string, string, error) {
	// Set command timeout using default timeout
	ctx, cancel := context.WithTimeout(context.Background(), GetDefaultTimeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, ffmpegPath, "-version")
	output, err := cmd.Output()
	if err != nil {
		return "", "", nil, "", FormatError("error getting FFmpeg version: %w", err)
	}

	fullOutput := string(output)
	version, configuration, libraries := extractVersionInfo(fullOutput)
	return version, configuration, libraries, fullOutput, nil
}

// Public functions (alphabetical)

// DetectFFmpeg identifies the FFmpeg installation on the system and its capabilities.
// It searches for the FFmpeg executable, checks its version, and determines if it supports
// advanced features like QP and CU analysis. The collected information is returned
// in an FFmpegInfo structure that applications can use to make decisions about
// available functionality.
func DetectFFmpeg() (*FFmpegInfo, error) {
	// Check if FFmpeg exists
	ffmpegPath, exists := checkFFmpegExistence()
	if !exists {
		return &FFmpegInfo{
			Installed: false,
		}, nil
	}

	// Get FFmpeg version
	version, _, _, versionOutput, err := getFFmpegVersion(ffmpegPath)
	if err != nil {
		return &FFmpegInfo{
			Installed: true,
			Path:      ffmpegPath,
		}, FormatError("FFmpeg found but error getting version: %w", err)
	}

	// Check if FFmpeg supports QP reading
	hasQPReading := checkQPReadingSupport(ffmpegPath, versionOutput)

	// Check if FFmpeg supports CU reading
	hasCUReading := checkCUReadingSupport(ffmpegPath, versionOutput)

	// Create the FFmpegInfo structure
	info := &FFmpegInfo{
		Installed:               true,
		Path:                    ffmpegPath,
		Version:                 version,
		HasQPReadingInfoSupport: hasQPReading,
		HasCUReadingInfoSupport: hasCUReading,
	}

	return info, nil
}

// FindFFmpeg is an alias for DetectFFmpeg to maintain compatibility with existing code.
// It identifies the FFmpeg installation on the system and its capabilities.
func FindFFmpeg() (*FFmpegInfo, error) {
	return DetectFFmpeg()
}

// extractVersion extracts the version number, configuration, and libraries from FFmpeg version output.
// It is used for tests to verify version parsing logic.
func extractVersion(versionOutput string) (string, string, []string) {
	version, configuration, libraries := extractVersionInfo(versionOutput)

	// Handle the case of empty or invalid input
	if version == "" {
		version = "unknown"
	}

	return version, configuration, libraries
}

// getCommonInstallPaths returns a list of common FFmpeg installation paths for the current OS.
// It provides possible locations where FFmpeg might be installed based on the operating system.
// The executable parameter specifies the executable name to look for (e.g., "ffmpeg" or "ffmpeg.exe").
func getCommonInstallPaths() []string {
	var execName string
	if runtime.GOOS == "windows" {
		execName = "ffmpeg.exe"
	} else {
		execName = "ffmpeg"
	}

	var searchPaths []string
	switch runtime.GOOS {
	case "windows":
		// Windows common paths
		searchPaths = []string{
			filepath.Join("C:", "Program Files", "FFmpeg", "bin", execName),
			filepath.Join("C:", "Program Files (x86)", "FFmpeg", "bin", execName),
			filepath.Join("C:", "FFmpeg", "bin", execName),
		}

		// Add ProgramFiles path if environment variable is set
		programFiles := os.Getenv("ProgramFiles")
		if programFiles != "" {
			searchPaths = append(searchPaths, filepath.Join(programFiles, "FFmpeg", "bin", execName))
		}

	case "darwin":
		// macOS common paths
		searchPaths = []string{
			filepath.Join("/usr", "local", "bin", execName),
			filepath.Join("/opt", "local", "bin", execName),
			filepath.Join("/opt", "homebrew", "bin", execName),
		}
	default:
		// Linux/Unix common paths
		searchPaths = []string{
			filepath.Join("/usr", "bin", execName),
			filepath.Join("/usr", "local", "bin", execName),
			filepath.Join("/opt", "ffmpeg", "bin", execName),
		}
	}
	return searchPaths
}
