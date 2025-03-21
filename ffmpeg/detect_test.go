package ffmpeg

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

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

// TestFindFFmpeg tests the FindFFmpeg function by verifying it can detect
// FFmpeg installation and properly initialize the FFmpegInfo struct.
func (s *FFmpegTestSuite) TestFindFFmpeg() {
	info, err := FindFFmpeg()
	require.NoError(s.T(), err, "Finding FFmpeg should not produce an error")

	// We can't guarantee FFmpeg is installed on the test system,
	// so we just log the results without failing the test
	s.T().Logf("FFmpeg installed: %v", info.Installed)

	// Verify that the FFmpegInfo struct is initialized correctly
	assert.NotNil(s.T(), info, "FFmpegInfo struct should not be nil")

	if info.Installed {
		s.T().Logf("FFmpeg path: %s", info.Path)
		s.T().Logf("FFmpeg version: %s", info.Version)
		s.T().Logf("FFmpeg QP reading support: %v", info.HasQPReadingInfoSupport)
		s.T().Logf("FFmpeg CU reading support: %v", info.HasCUReadingInfoSupport)

		// If installed, verify that the path exists
		_, err := os.Stat(info.Path)
		assert.NoError(s.T(), err, "FFmpeg path should exist on the system")

		// Verify that the version is not unknown
		assert.NotEqual(s.T(), "unknown", info.Version, "FFmpeg version should be detected")
	} else {
		// Even if not installed, fields should be initialized to their zero values
		assert.Empty(s.T(), info.Path, "Path should be empty when FFmpeg is not installed")
		assert.Equal(s.T(), "unknown", info.Version, "Version should be 'unknown' when FFmpeg is not installed")
		assert.False(s.T(), info.HasQPReadingInfoSupport, "HasQPReadingInfoSupport should be false when FFmpeg is not installed")
		assert.False(s.T(), info.HasCUReadingInfoSupport, "HasCUReadingInfoSupport should be false when FFmpeg is not installed")
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
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := extractVersion(tc.input)
			assert.Equal(s.T(), tc.expected, result)
		})
	}
}

// TestGetCommonInstallPaths tests the getCommonInstallPaths function to ensure
// it returns appropriate installation paths for different operating systems.
func (s *FFmpegTestSuite) TestGetCommonInstallPaths() {
	// Test for Windows
	if runtime.GOOS == "windows" {
		paths := getCommonInstallPaths("ffmpeg.exe")
		assert.NotEmpty(s.T(), paths)

		// Check that paths use proper path joining
		for _, path := range paths {
			assert.True(s.T(), filepath.IsAbs(path), "Path should be absolute: %s", path)
		}

		// Check for common Windows paths
		programFiles := os.Getenv("ProgramFiles")
		if programFiles != "" {
			expectedPath := filepath.Join(programFiles, "FFmpeg", "bin", "ffmpeg.exe")
			assert.Contains(s.T(), paths, expectedPath)
		}
	}

	// Test for macOS
	if runtime.GOOS == "darwin" {
		paths := getCommonInstallPaths("ffmpeg")
		assert.NotEmpty(s.T(), paths)

		// Check that paths use proper path joining
		for _, path := range paths {
			assert.True(s.T(), filepath.IsAbs(path), "Path should be absolute: %s", path)
		}

		// Check for common macOS paths
		assert.Contains(s.T(), paths, filepath.Join("/usr", "local", "bin", "ffmpeg"))
		assert.Contains(s.T(), paths, filepath.Join("/opt", "homebrew", "bin", "ffmpeg"))
	}

	// Test for Linux
	if runtime.GOOS == "linux" {
		paths := getCommonInstallPaths("ffmpeg")
		assert.NotEmpty(s.T(), paths)

		// Check that paths use proper path joining
		for _, path := range paths {
			assert.True(s.T(), filepath.IsAbs(path), "Path should be absolute: %s", path)
		}

		// Check for common Linux paths
		assert.Contains(s.T(), paths, filepath.Join("/usr", "bin", "ffmpeg"))
		assert.Contains(s.T(), paths, filepath.Join("/usr", "local", "bin", "ffmpeg"))
	}

	// Test for other OS (should still return some default paths)
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		paths := getCommonInstallPaths("ffmpeg")
		assert.NotEmpty(s.T(), paths)
	}
}

// TestCheckQPReadingSupport tests the checkQPReadingSupport function to ensure
// it correctly identifies FFmpeg builds with QP reading support.
func (s *FFmpegTestSuite) TestCheckQPReadingSupport() {
	// Skip if FFmpeg is not installed
	info, err := FindFFmpeg()
	require.NoError(s.T(), err, "Finding FFmpeg should not produce an error")
	if !info.Installed {
		s.T().Skip("FFmpeg not installed, skipping test")
	}

	// Test cases for specific version outputs without dependency on actual FFmpeg
	testCases := []struct {
		name    string
		version string
		expect  bool
	}{
		{
			name:    "With debug flag",
			version: "ffmpeg version 4.2.7 --enable-debug",
			expect:  true,
		},
		{
			name:    "With debug build info",
			version: "ffmpeg version 4.2.7 built with-debug",
			expect:  true,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			mockPath := "mock/ffmpeg" // Use a mock path to avoid testing actual file existence
			result := checkQPReadingSupport(mockPath, tc.version)
			assert.Equal(s.T(), tc.expect, result, "QP support detection should match expected value for %s", tc.name)
		})
	}

	// Test with actual version output from the installed FFmpeg
	cmd := exec.Command(info.Path, "-version")
	output, err := cmd.Output()
	require.NoError(s.T(), err, "Getting FFmpeg version should not produce an error")

	result := checkQPReadingSupport(info.Path, string(output))
	s.T().Logf("QP support detection with actual version output: %v", result)
	// We don't assert on the result as it depends on the actual FFmpeg installation
}

// TestCheckCUReadingSupport tests the checkCUReadingSupport function to ensure
// it correctly identifies FFmpeg builds with CU reading support.
func (s *FFmpegTestSuite) TestCheckCUReadingSupport() {
	// Skip if FFmpeg is not installed
	info, err := FindFFmpeg()
	require.NoError(s.T(), err, "Finding FFmpeg should not produce an error")
	if !info.Installed {
		s.T().Skip("FFmpeg not installed, skipping test")
	}

	// Test cases for specific version outputs without dependency on actual FFmpeg
	testCases := []struct {
		name    string
		version string
		expect  bool
	}{
		{
			name:    "With debug flag",
			version: "ffmpeg version 4.2.7 --enable-debug",
			expect:  true,
		},
		{
			name:    "With debug build info",
			version: "ffmpeg version 4.2.7 built with-debug",
			expect:  true,
		},
	}

	// Run test cases
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			mockPath := "mock/ffmpeg" // Use a mock path to avoid testing actual file existence
			result := checkCUReadingSupport(mockPath, tc.version)
			assert.Equal(s.T(), tc.expect, result, "CU support detection should match expected value for %s", tc.name)
		})
	}

	// Test with actual version output from the installed FFmpeg
	cmd := exec.Command(info.Path, "-version")
	output, err := cmd.Output()
	require.NoError(s.T(), err, "Getting FFmpeg version should not produce an error")

	result := checkCUReadingSupport(info.Path, string(output))
	s.T().Logf("CU support detection with actual version output: %v", result)
	// We don't assert on the result as it depends on the actual FFmpeg installation
}

// TestFFmpegSuite runs the FFmpeg test suite.
func TestFFmpegSuite(t *testing.T) {
	suite.Run(t, new(FFmpegTestSuite))
}
