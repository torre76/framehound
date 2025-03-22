package main

import (
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
	// Create a temporary directory for the test output
	outputDir := filepath.Join(s.tempDir, "output")
	err := os.MkdirAll(outputDir, 0755)
	s.NoError(err)

	// Create a test container info
	containerInfo := &ffmpeg.ContainerInfo{
		General: ffmpeg.GeneralInfo{
			Format:    "MPEG-4",
			Size:      "1234567 bytes",
			Duration:  "120.5 s",
			DurationF: 120.5,
			BitRate:   "5000000 b/s",
			Tags: map[string]string{
				"file_path": "/path/to/test.mp4",
			},
		},
		VideoStreams: []ffmpeg.VideoStream{
			{
				Format:             "H.264",
				FormatProfile:      "High",
				Width:              1920,
				Height:             1080,
				DisplayAspectRatio: 1.78,
				BitRate:            10000000,
				FrameRate:          30.0,
				BitDepth:           8,
				ScanType:           "Progressive",
				ColorSpace:         "YUV",
			},
		},
		AudioStreams: []ffmpeg.AudioStream{
			{
				Format:        "AAC",
				Channels:      2,
				ChannelLayout: "L R",
				SamplingRate:  48000,
				BitRate:       192000,
				Language:      "eng",
			},
		},
		SubtitleStreams: []ffmpeg.SubtitleStream{
			{
				Format:   "SRT",
				Language: "eng",
				Title:    "English",
			},
		},
	}

	// Save the container info
	err = saveContainerInfo(containerInfo, outputDir)
	s.NoError(err)

	// Verify that the file was created
	infoFile := filepath.Join(outputDir, "test_info.json")
	_, err = os.Stat(infoFile)
	s.NoError(err)

	// Read the file and verify its contents
	content, err := os.ReadFile(infoFile)
	s.NoError(err)
	s.Contains(string(content), "MPEG-4")
	s.Contains(string(content), "1234567")
	s.Contains(string(content), "120.5")
	s.Contains(string(content), "H.264")
	s.Contains(string(content), "1920")
	s.Contains(string(content), "AAC")
}

// TestMainTestSuite runs the test suite.
func TestMainTestSuite(t *testing.T) {
	suite.Run(t, new(MainTestSuite))
}
