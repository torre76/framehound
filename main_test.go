package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/torre76/framehound/ffmpeg"
)

// MainTestSuite defines a test suite for the main package functionality.
type MainTestSuite struct {
	suite.Suite
	tempDir string // Temporary directory for test files
}

// SetupSuite prepares the test environment by creating a temporary directory.
func (s *MainTestSuite) SetupSuite() {
	// Save original color setting and disable color for tests
	originalNoColor := color.NoColor
	color.NoColor = true

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "framehound-test")
	require.NoError(s.T(), err)
	s.tempDir = tempDir

	// Restore color setting in TearDownSuite
	s.T().Cleanup(func() {
		color.NoColor = originalNoColor
	})
}

// TearDownSuite cleans up the test environment by removing the temporary directory.
func (s *MainTestSuite) TearDownSuite() {
	// Clean up temporary directory
	os.RemoveAll(s.tempDir)
}

// TestFormatWithThousandSeparators tests the formatWithThousandSeparators function
// to ensure it correctly formats integers with thousand separators.
func (s *MainTestSuite) TestFormatWithThousandSeparators() {
	testCases := []struct {
		name     string
		input    int64
		expected string
	}{
		{
			name:     "zero",
			input:    0,
			expected: "0",
		},
		{
			name:     "single_digit",
			input:    5,
			expected: "5",
		},
		{
			name:     "two_digits",
			input:    42,
			expected: "42",
		},
		{
			name:     "three_digits",
			input:    123,
			expected: "123",
		},
		{
			name:     "four_digits",
			input:    1234,
			expected: "1,234",
		},
		{
			name:     "five_digits",
			input:    12345,
			expected: "12,345",
		},
		{
			name:     "six_digits",
			input:    123456,
			expected: "123,456",
		},
		{
			name:     "seven_digits",
			input:    1234567,
			expected: "1,234,567",
		},
		{
			name:     "negative_four_digits",
			input:    -1234,
			expected: "-1,234",
		},
		{
			name:     "negative_seven_digits",
			input:    -1234567,
			expected: "-1,234,567",
		},
		{
			name:     "billion",
			input:    1000000000,
			expected: "1,000,000,000",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := formatWithThousandSeparators(tc.input)
			assert.Equal(s.T(), tc.expected, result)
		})
	}
}

// TestSaveContainerInfo tests the saveContainerInfo function to ensure
// it correctly saves container information to a file.
func (s *MainTestSuite) TestSaveContainerInfo() {
	// Create a test container info
	containerInfo := &ffmpeg.ContainerInfo{
		General: ffmpeg.GeneralInfo{
			CompleteName:   "/path/to/test.mp4",
			Format:         "MPEG-4",
			FormatVersion:  "Version 2",
			FileSize:       1234567,
			Duration:       120.5,
			OverallBitRate: 5000000,
			FrameRate:      30.0,
		},
		VideoStreams: []ffmpeg.VideoStream{
			{
				Format:             "H.264",
				FormatProfile:      "High",
				Width:              1920,
				Height:             1080,
				DisplayAspectRatio: 1.778,
				BitDepth:           8,
				BitRate:            4000000,
				FrameRate:          30.0,
				ScanType:           "Progressive",
				ColorSpace:         "YUV",
			},
		},
	}

	// Create output directory
	outputDir := filepath.Join(s.tempDir, "container-info-test")
	err := os.MkdirAll(outputDir, 0755)
	require.NoError(s.T(), err)

	// Call the function being tested
	err = saveContainerInfo(containerInfo, outputDir)
	assert.NoError(s.T(), err)

	// Verify file was created
	outputFile := filepath.Join(outputDir, "info.txt")
	_, err = os.Stat(outputFile)
	assert.NoError(s.T(), err)

	// Read the file content
	content, err := os.ReadFile(outputFile)
	require.NoError(s.T(), err)

	// Check content contains expected information
	contentStr := string(content)
	assert.Contains(s.T(), contentStr, "Container Information")
	assert.Contains(s.T(), contentStr, "/path/to/test.mp4")
	assert.Contains(s.T(), contentStr, "MPEG-4 Version 2")
	assert.Contains(s.T(), contentStr, "1,234,567 bytes")
	assert.Contains(s.T(), contentStr, "H.264 (High)")
	assert.Contains(s.T(), contentStr, "FrameHound Version")
}

// captureOutput captures stdout during the execution of a function
// and returns the captured output as a string.
func captureOutput(fn func()) string {
	// Save original stdout
	oldStdout := os.Stdout

	// Create a new file for stdout that writes to our buffer
	newStdout, _ := os.CreateTemp("", "stdout")
	os.Stdout = newStdout

	// Call the function that produces output
	fn()

	// Get the output
	newStdout.Seek(0, 0)
	output, _ := io.ReadAll(newStdout)

	// Clean up and restore original stdout
	newStdout.Close()
	os.Remove(newStdout.Name())
	os.Stdout = oldStdout

	// Return captured output
	return string(output)
}

// TestMainTestSuite runs the test suite.
func TestMainTestSuite(t *testing.T) {
	suite.Run(t, new(MainTestSuite))
}
