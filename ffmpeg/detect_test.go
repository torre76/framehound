// Package ffmpeg provides functionality for detecting and working with FFmpeg.
// This file contains tests for the FFmpeg detection functionality.
// It tests detection, version extraction, and support for QP reading.
package ffmpeg

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// FFmpegTestSuite defines a test suite for FFmpeg functionality.
// It tests detection, version extraction, and support for QP and CU reading.
type FFmpegTestSuite struct {
	suite.Suite
	tempDir string // Temporary directory for test files
}

// SetupSuite prepares the test environment by creating a temporary directory.
func (s *FFmpegTestSuite) SetupSuite() {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "ffmpeg-test")
	require.NoError(s.T(), err)
	s.tempDir = tempDir
}

// TearDownSuite cleans up the test environment by removing the temporary directory.
func (s *FFmpegTestSuite) TearDownSuite() {
	// Clean up temporary directory
	os.RemoveAll(s.tempDir)
}

// TestFindFFmpeg tests finding FFmpeg installation
func (s *FFmpegTestSuite) TestFindFFmpeg() {
	// Get FFmpeg info
	info, err := FindFFmpeg()
	require.NoError(s.T(), err, "FindFFmpeg() should not return error")

	// Log the info
	s.T().Logf("FFmpeg installed: %v", info.Installed)
	if info.Installed {
		s.T().Logf("FFmpeg path: %s", info.Path)
		s.T().Logf("FFmpeg version: %s", info.Version)
		s.T().Logf("FFmpeg QP reading support: %v", info.HasQPReadingInfoSupport)
	}

	// Make assertions
	if info.Installed {
		assert.NotEmpty(s.T(), info.Path, "Path should not be empty when FFmpeg is installed")
		assert.NotEmpty(s.T(), info.Version, "Version should not be empty when FFmpeg is installed")
	} else {
		assert.Empty(s.T(), info.Path, "Path should be empty when FFmpeg is not installed")
		assert.Equal(s.T(), "", info.Version, "Version should be empty when FFmpeg is not installed")
		assert.False(s.T(), info.HasQPReadingInfoSupport, "HasQPReadingInfoSupport should be false when FFmpeg is not installed")
	}
}

// TestExtractVersion tests the extractVersion function with various input formats
// to ensure it correctly parses FFmpeg version information.
func (s *FFmpegTestSuite) TestExtractVersion() {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal output",
			input:    "ffmpeg version 4.2.7 Copyright (c) 2000-2022 the FFmpeg developers",
			expected: "4.2.7",
		},
		{
			name:     "empty output",
			input:    "",
			expected: "unknown",
		},
		{
			name:     "malformed output",
			input:    "ffmpeg",
			expected: "unknown",
		},
		{
			name:     "multiline output",
			input:    "ffmpeg version 5.0.1 Copyright (c) 2000-2022 the FFmpeg developers\nbuilt with gcc 11.2.0",
			expected: "5.0.1",
		},
		{
			name:     "missing version",
			input:    "ffmpeg Copyright (c) 2000-2022 the FFmpeg developers",
			expected: "(c)",
		},
		{
			name:     "git version format",
			input:    "ffmpeg version n4.4.1 Copyright (c) 2000-2021 the FFmpeg developers",
			expected: "4.4.1",
		},
		{
			name:     "development version format",
			input:    "ffmpeg version 6.1-dev-3246 Copyright (c) 2000-2023 the FFmpeg developers",
			expected: "6.1",
		},
		{
			name:     "capital letter format",
			input:    "FFmpeg version 4.2.7 Copyright (c) 2000-2022 the FFmpeg developers",
			expected: "4.2.7",
		},
		{
			name:     "version with extra digits",
			input:    "ffmpeg version 5.1.2.3 Copyright (c) 2000-2022 the FFmpeg developers",
			expected: "5.1.2.3",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result, _, _ := extractVersion(tc.input)
			assert.Equal(s.T(), tc.expected, result)
		})
	}
}

// TestGetCommonInstallPaths tests the getCommonInstallPaths function to ensure
// it returns appropriate installation paths for different operating systems.
func (s *FFmpegTestSuite) TestGetCommonInstallPaths() {
	paths := getCommonInstallPaths()
	assert.NotEmpty(s.T(), paths)

	// Check that paths are appropriate for the current OS
	if runtime.GOOS == "windows" {
		// Check for .exe extension in Windows paths
		for _, path := range paths {
			assert.True(s.T(), filepath.IsAbs(path), "Path should be absolute: %s", path)
			assert.Contains(s.T(), path, "ffmpeg.exe")
		}
	} else {
		// Check for 'ffmpeg' executable in Unix paths
		for _, path := range paths {
			assert.True(s.T(), filepath.IsAbs(path), "Path should be absolute: %s", path)
			assert.Contains(s.T(), path, "ffmpeg")
		}
	}

	// Check for common paths based on OS
	switch runtime.GOOS {
	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		if programFiles != "" {
			expectedPath := filepath.Join(programFiles, "FFmpeg", "bin", "ffmpeg.exe")
			assert.Contains(s.T(), paths, expectedPath)
		}
	case "darwin":
		assert.Contains(s.T(), paths, filepath.Join("/usr", "local", "bin", "ffmpeg"))
		assert.Contains(s.T(), paths, filepath.Join("/opt", "homebrew", "bin", "ffmpeg"))
	case "linux":
		assert.Contains(s.T(), paths, filepath.Join("/usr", "bin", "ffmpeg"))
		assert.Contains(s.T(), paths, filepath.Join("/usr", "local", "bin", "ffmpeg"))
	}
}

// TestCheckQPReadingSupport tests the QP reading support detection function
func (s *FFmpegTestSuite) TestCheckQPReadingSupport() {
	// Only run test for QP support detection if FFmpeg is installed
	ffmpegInstalled := false
	var info *FFmpegInfo
	var err error

	// Detect FFmpeg
	info, err = FindFFmpeg()
	if err == nil && info.Installed {
		ffmpegInstalled = true
	}

	// Test cases for QP support detection
	testCases := []struct {
		name    string
		version string
		expect  bool
	}{
		{
			name:    "With debug flag",
			version: "ffmpeg version 4.2.7 Copyright (c) 2000-2019 the FFmpeg developers\nbuilt with gcc 9.3.0 (Debian 9.3.0-15) 20200512\nconfiguration: --enable-debug",
			expect:  true,
		},
		{
			name:    "With debug build info",
			version: "ffmpeg version 4.2.7 Copyright (c) 2000-2019 the FFmpeg developers\nbuilt with gcc 9.3.0 (Debian 9.3.0-15) 20200512\nconfiguration: --prefix=/usr --extra-version=1build1 --toolchain=hardened --libdir=/usr/lib/x86_64-linux-gnu --incdir=/usr/include/x86_64-linux-gnu --enable-debug=3",
			expect:  true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Skip test if FFmpeg is not installed
			if !ffmpegInstalled {
				s.T().Skip("FFmpeg not installed, skipping test")
				return
			}

			hasSupport := checkQPReadingSupport(info.Path, tc.version)
			assert.Equal(s.T(), tc.expect, hasSupport)
		})
	}

	// Only run the actual FFmpeg version detection if FFmpeg is installed
	if ffmpegInstalled {
		s.T().Logf("QP support detection with actual version output: %v", info.HasQPReadingInfoSupport)
	}
}

// TestGetFFmpegPath tests the GetFFmpegPath function to ensure it correctly
// detects the FFmpeg executable path.
func (s *FFmpegTestSuite) TestGetFFmpegPath() {
	// Get FFmpeg path
	path := GetFFmpegPath()

	// Check if FFmpeg is installed
	info, _ := FindFFmpeg()

	if info.Installed {
		assert.NotEmpty(s.T(), path, "Path should not be empty when FFmpeg is installed")
		assert.FileExists(s.T(), path, "FFmpeg executable should exist at the reported path")
	} else {
		assert.Empty(s.T(), path, "Path should be empty when FFmpeg is not installed")
	}
}

// TestGetExecutablePaths tests the GetExecutablePaths function to ensure it
// correctly returns both FFmpeg and FFprobe paths.
func (s *FFmpegTestSuite) TestGetExecutablePaths() {
	// Skip test if FFmpeg is not installed
	info, _ := FindFFmpeg()
	if !info.Installed {
		s.T().Skip("FFmpeg not installed, skipping test")
		return
	}

	// Get executable paths
	paths := GetExecutablePaths(info.Path)

	// Verify FFmpeg path
	assert.Equal(s.T(), info.Path, paths.FFmpeg, "FFmpeg path should match the detected path")

	// Verify FFprobe path exists in the same directory
	ffprobeDir := filepath.Dir(paths.FFprobe)
	ffmpegDir := filepath.Dir(paths.FFmpeg)
	assert.Equal(s.T(), ffmpegDir, ffprobeDir, "FFprobe should be in the same directory as FFmpeg")

	// Check correct extension based on OS
	if runtime.GOOS == "windows" {
		assert.True(s.T(), strings.HasSuffix(paths.FFprobe, ".exe"), "FFprobe path should have .exe extension on Windows")
	} else {
		assert.False(s.T(), strings.HasSuffix(paths.FFprobe, ".exe"), "FFprobe path should not have .exe extension on non-Windows")
	}
}

// TestVerifyFFmpeg tests the VerifyFFmpeg function to ensure it correctly
// verifies FFmpeg installation and returns the proper information.
func (s *FFmpegTestSuite) TestVerifyFFmpeg() {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Verify FFmpeg
	info, err := VerifyFFmpeg(ctx)
	require.NoError(s.T(), err, "VerifyFFmpeg() should not return error")

	// Compare with FindFFmpeg results
	findInfo, _ := FindFFmpeg()

	// Check if results are consistent
	assert.Equal(s.T(), findInfo.Installed, info.Installed, "Installed status should be consistent")

	if info.Installed {
		assert.Equal(s.T(), findInfo.Path, info.Path, "Path should be consistent")
		assert.NotEmpty(s.T(), info.Version, "Version should not be empty when FFmpeg is installed")
	}
}

// TestFFmpegSuite runs the FFmpeg test suite.
func TestFFmpegSuite(t *testing.T) {
	suite.Run(t, new(FFmpegTestSuite))
}
