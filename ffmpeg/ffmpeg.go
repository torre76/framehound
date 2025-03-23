// Package ffmpeg provides functionality for detecting and working with FFmpeg.
// It offers capabilities for analyzing video files, extracting metadata, and
// processing frame-level information such as bitrates, quality parameters, and
// quality metrics including QP values, PSNR, SSIM, and VMAF.
package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Private variables (alphabetical)

// ffmpegVersionRegex is used to detect FFmpeg version from version string.
// It extracts the numeric version (e.g., 4.4.1) from FFmpeg's version output.
var ffmpegVersionRegex = regexp.MustCompile(`version\s+(\d+\.\d+(?:\.\d+)?)`)

// Private functions (alphabetical)

// Public functions (alphabetical)

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
	// Check OS-specific locations based on typical installations
	var possiblePaths []string
	switch runtime.GOOS {
	case "windows":
		possiblePaths = []string{
			"ffmpeg.exe",
			"C:\\Program Files\\FFmpeg\\bin\\ffmpeg.exe",
			"C:\\Program Files (x86)\\FFmpeg\\bin\\ffmpeg.exe",
		}
	case "darwin":
		possiblePaths = []string{
			"/usr/local/bin/ffmpeg",
			"/usr/bin/ffmpeg",
			"/opt/homebrew/bin/ffmpeg",
			"/opt/local/bin/ffmpeg",
		}
	default: // Linux and others
		possiblePaths = []string{
			"/usr/bin/ffmpeg",
			"/usr/local/bin/ffmpeg",
			"/opt/ffmpeg/bin/ffmpeg",
		}
	}

	// Try PATH first (will work for all OS if ffmpeg is in PATH)
	pathCmd := exec.Command("which", "ffmpeg")
	if runtime.GOOS == "windows" {
		pathCmd = exec.Command("where", "ffmpeg")
	}

	out, err := pathCmd.Output()
	if err == nil && len(out) > 0 {
		path := strings.TrimSpace(string(out))
		if strings.Contains(path, "\n") {
			path = strings.Split(path, "\n")[0] // Take first match
		}
		return path
	}

	// Try specific paths
	for _, path := range possiblePaths {
		cmd := exec.Command(path, "-version")
		err := cmd.Run()
		if err == nil {
			return path
		}
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
	matches := ffmpegVersionRegex.FindStringSubmatch(output)
	if len(matches) >= 2 {
		version = matches[1]
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
