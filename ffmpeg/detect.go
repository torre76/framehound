// Package ffmpeg provides functionality for detecting and working with FFmpeg.
// It includes capabilities for detecting FFmpeg installation, version, and support
// for advanced features like QP analysis.
package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Private variables (alphabetical)

// ffmpegVersionRegex is used to detect FFmpeg version from version string.
// It extracts the numeric version (e.g., 4.4.1) from FFmpeg's version output.
var ffmpegVersionRegex = regexp.MustCompile(`(?i)(?:version|ffmpeg)\s+(?:n|\w)?(\d+\.\d+(?:\.\d+(?:\.\d+)?)?)`)

// Private functions (alphabetical)

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

// extractVersionInfo parses FFmpeg's version output to extract version, configuration and libraries.
// It breaks down the raw version output into structured components, making it easier to
// understand the capabilities of the installed FFmpeg instance.
func extractVersionInfo(versionOutput string) (string, string, []string) {
	lines := strings.Split(versionOutput, "\n")
	if len(lines) == 0 {
		return "", "", nil
	}

	version := parseVersionFromFirstLine(lines[0])
	configuration := extractConfiguration(lines)
	libraries := extractLibraries(lines)

	return version, configuration, libraries
}

// parseVersionFromFirstLine parses the version string from the first line of FFmpeg output.
func parseVersionFromFirstLine(firstLine string) string {
	versionParts := strings.Split(firstLine, " version ")
	if len(versionParts) > 1 {
		remainingParts := strings.Split(versionParts[1], " ")
		if len(remainingParts) > 0 {
			// Extract only the version part (handle 'n' prefix and '-dev' suffix)
			versionStr := remainingParts[0]

			// Remove 'n' prefix if present (git versioning)
			versionStr = strings.TrimPrefix(versionStr, "n")

			// Remove development suffix if present (e.g., -dev-1234)
			if idx := strings.Index(versionStr, "-dev"); idx > 0 {
				versionStr = versionStr[:idx]
			}

			return versionStr
		}
	}

	// If no version found but contains (c), extract that instead
	if strings.Contains(firstLine, "(c)") {
		return "(c)"
	}

	return ""
}

// extractConfiguration finds the configuration line in FFmpeg output.
func extractConfiguration(lines []string) string {
	for _, line := range lines {
		if strings.HasPrefix(line, "  configuration:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "  configuration:"))
		}
	}
	return ""
}

// extractLibraries parses the libraries section from FFmpeg output.
func extractLibraries(lines []string) []string {
	var libraries []string
	capturingLibraries := false

	for _, line := range lines {
		// Start capturing when we see a line starting with "libavutil" or "lib"
		if strings.HasPrefix(line, "  libavutil") || strings.HasPrefix(line, "  lib") {
			capturingLibraries = true
		}

		// Capture library lines
		if capturingLibraries && strings.HasPrefix(line, "  lib") {
			libraries = append(libraries, strings.TrimSpace(line))
		}

		// Stop when we hit a non-empty line that doesn't start with "  lib" after we've started capturing
		if capturingLibraries && !strings.HasPrefix(line, "  lib") && line != "" {
			capturingLibraries = false
		}
	}

	return libraries
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

// DetectFFmpeg locates and identifies FFmpeg installation on the system.
// It returns an FFmpegInfo struct with details including path, version, and capabilities.
// If FFmpeg is not installed or cannot be found, it returns an error.
func DetectFFmpeg() (*FFmpegInfo, error) {
	// Find the FFmpeg executable
	ffmpegPath, found := checkFFmpegExistence()
	if !found {
		return &FFmpegInfo{
			Installed: false,
		}, nil
	}

	// Get FFmpeg version information
	version, _, _, versionOutput, err := getFFmpegVersion(ffmpegPath)
	if err != nil {
		return &FFmpegInfo{
			Installed: false,
		}, err
	}

	// If version is empty, set it to unknown
	if version == "" {
		version = "unknown"
	}

	// Check if FFmpeg supports QP reading
	hasQPReading := checkQPReadingSupport(ffmpegPath, versionOutput)

	// Create the FFmpegInfo structure
	info := &FFmpegInfo{
		Installed:               true,
		Path:                    ffmpegPath,
		Version:                 version,
		HasQPReadingInfoSupport: hasQPReading,
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

// GetExecutablePaths gets the paths to both FFmpeg and FFprobe.
// It assumes FFprobe is located in the same directory as FFmpeg.
func GetExecutablePaths(ffmpegPath string) *ExecutablePaths {
	ffprobePath := filepath.Join(filepath.Dir(ffmpegPath), "ffprobe")
	if runtime.GOOS == "windows" {
		ffprobePath += ".exe"
	}
	return &ExecutablePaths{
		FFmpeg:  ffmpegPath,
		FFprobe: ffprobePath,
	}
}

// GetFFmpegPath attempts to detect the path to the FFmpeg executable.
// It returns the path if found, or an empty string if not found.
func GetFFmpegPath() string {
	ffmpegPath, found := checkFFmpegExistence()
	if found {
		return ffmpegPath
	}
	return ""
}

// VerifyFFmpeg checks if FFmpeg is installed and available.
// It returns a populated FFmpegInfo struct with installation details.
func VerifyFFmpeg(ctx context.Context) (*FFmpegInfo, error) {
	// Try to find FFmpeg
	ffmpegPath := GetFFmpegPath()
	if ffmpegPath == "" {
		return &FFmpegInfo{
			Installed: false,
		}, nil
	}

	// Execute FFmpeg to get version
	cmd := exec.CommandContext(ctx, ffmpegPath, "-version")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return &FFmpegInfo{
			Path:      ffmpegPath,
			Installed: false,
		}, fmt.Errorf("failed to execute FFmpeg: %w", err)
	}

	// Extract version number
	version := ""
	output := out.String()

	// Try to extract using regex first
	matches := ffmpegVersionRegex.FindStringSubmatch(output)
	if len(matches) >= 2 {
		version = matches[1]
	}

	// If version is still empty, try fallback method by parsing the first line
	if version == "" {
		lines := strings.Split(output, "\n")
		if len(lines) > 0 {
			version = parseVersionFromFirstLine(lines[0])
		}
	}

	// If version is still empty but we have output, use a generic version
	if version == "" && len(output) > 0 {
		version = "detected"
	}

	// Check if FFmpeg can provide QP information
	// This is detected based on the presence of debug options
	hasQPSupport := strings.Contains(output, "debug")

	return &FFmpegInfo{
		Path:                    ffmpegPath,
		Version:                 version,
		Installed:               true,
		HasQPReadingInfoSupport: hasQPSupport,
	}, nil
}
