package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// Add containerInfo for testing
	testContainerInfo *ffmpeg.ContainerInfo
	prober            *ffmpeg.Prober
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

	// Create a real prober for testing
	ffmpegInfo := &ffmpeg.FFmpegInfo{
		Installed: true,
		Path:      "/usr/bin/ffmpeg", // This path doesn't need to be real for testing
		Version:   "Test Version",
	}
	prober, err := ffmpeg.NewProber(ffmpegInfo)
	require.NoError(s.T(), err)
	s.prober = prober

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

// SetupTest prepares each test by creating a test container info object.
func (s *MainTestSuite) SetupTest() {
	// Create a test container info for use in multiple tests
	s.testContainerInfo = &ffmpeg.ContainerInfo{
		General: ffmpeg.GeneralInfo{
			Format:    "MPEG-4",
			Size:      "1234567 bytes",
			Duration:  "120.5 s",
			DurationF: 120.5,
			BitRate:   "5000000 b/s",
			Tags: map[string]string{
				"file_path": "/path/to/test.mp4",
				"title":     "Test Movie", // Add a title for the prober to use
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

// TestPrintSimpleContainerSummary tests the printSimpleContainerSummary function to ensure
// it properly outputs the expected information.
// This is a non-assertion test as it primarily tests output formatting,
// which is difficult to assert programmatically.
func (s *MainTestSuite) TestPrintSimpleContainerSummary() {
	// Since we're testing a function that outputs to stdout,
	// this is primarily to ensure it doesn't panic.
	// Create test cases with varying numbers of streams
	testCases := []struct {
		name          string
		videoCount    int
		audioCount    int
		subtitleCount int
	}{
		{
			name:          "single_streams",
			videoCount:    1,
			audioCount:    1,
			subtitleCount: 1,
		},
		{
			name:          "plural_streams",
			videoCount:    2,
			audioCount:    3,
			subtitleCount: 4,
		},
		{
			name:          "no_streams",
			videoCount:    0,
			audioCount:    0,
			subtitleCount: 0,
		},
		{
			name:          "mixed_streams",
			videoCount:    1,
			audioCount:    0,
			subtitleCount: 3,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Create a custom container info with the specified stream counts
			customInfo := *s.testContainerInfo

			// Adjust video streams
			customInfo.VideoStreams = customInfo.VideoStreams[:0]
			for i := 0; i < tc.videoCount; i++ {
				customInfo.VideoStreams = append(customInfo.VideoStreams, s.testContainerInfo.VideoStreams[0])
			}

			// Adjust audio streams
			customInfo.AudioStreams = customInfo.AudioStreams[:0]
			for i := 0; i < tc.audioCount; i++ {
				customInfo.AudioStreams = append(customInfo.AudioStreams, s.testContainerInfo.AudioStreams[0])
			}

			// Adjust subtitle streams
			customInfo.SubtitleStreams = customInfo.SubtitleStreams[:0]
			for i := 0; i < tc.subtitleCount; i++ {
				customInfo.SubtitleStreams = append(customInfo.SubtitleStreams, s.testContainerInfo.SubtitleStreams[0])
			}

			// Call the function - this primarily tests that it doesn't panic
			// Since we've disabled colors for testing, this won't produce colorized output
			printSimpleContainerSummary(&customInfo, s.prober)

			// No explicit assertions as we're testing stdout formatting
			// The test passes if the function completes without panicking
		})
	}
}

// TestSaveContainerInfo tests the saveContainerInfo function to ensure it correctly saves
// container information to a JSON file.
func (s *MainTestSuite) TestSaveContainerInfo() {
	// Create a temporary directory for the test output
	outputDir := filepath.Join(s.tempDir, "container_info_output")
	err := os.MkdirAll(outputDir, 0755)
	s.NoError(err)

	// Call the saveContainerInfo function with the test container info
	err = saveContainerInfo(s.testContainerInfo, outputDir)
	s.NoError(err)

	// Verify that the file was created
	infoFile := filepath.Join(outputDir, "test_info.json")
	_, err = os.Stat(infoFile)
	s.NoError(err)

	// Read the file and verify its contents
	content, err := os.ReadFile(infoFile)
	s.NoError(err)

	// Deserialize the JSON and verify expected content
	var deserializedInfo map[string]interface{}
	err = json.Unmarshal(content, &deserializedInfo)
	s.NoError(err)

	// Check the filename contains "test.mp4"
	s.Contains(deserializedInfo["filename"], "test.mp4")

	// Check format information
	formatInfo, ok := deserializedInfo["format"].(map[string]interface{})
	s.True(ok)
	s.Equal("MPEG-4", formatInfo["name"])
	s.Equal(120.5, formatInfo["duration"])

	// Check streams
	s.NotNil(deserializedInfo["video_streams"])
	s.NotNil(deserializedInfo["audio_streams"])
	s.NotNil(deserializedInfo["subtitle_streams"])
}

// TestParseBitRate tests the parseBitRate function to ensure
// it correctly parses bitrate strings into integer values.
func (s *MainTestSuite) TestParseBitRate() {
	testCases := []struct {
		name     string
		input    string
		expected int64
	}{
		{
			name:     "empty_string",
			input:    "",
			expected: 0,
		},
		{
			name:     "simple_bps",
			input:    "1000",
			expected: 1000,
		},
		{
			name:     "kbps",
			input:    "1000 kb/s",
			expected: 1000000,
		},
		{
			name:     "mbps",
			input:    "5 Mb/s",
			expected: 5000000,
		},
		{
			name:     "with_spaces",
			input:    "5000 b/s",
			expected: 5000,
		},
		{
			name:     "invalid_format",
			input:    "not a number",
			expected: 0,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := parseBitRate(tc.input)
			assert.Equal(s.T(), tc.expected, result)
		})
	}
}

// TestGetFrameRate tests the getFrameRate function to ensure
// it correctly extracts the frame rate from a container info.
func (s *MainTestSuite) TestGetFrameRate() {
	// Test with container having video streams
	s.Run("with_video_streams", func() {
		frameRate := getFrameRate(s.testContainerInfo)
		assert.Equal(s.T(), 30.0, frameRate)
	})

	// Test with container having no video streams
	s.Run("no_video_streams", func() {
		emptyInfo := &ffmpeg.ContainerInfo{
			General:      s.testContainerInfo.General,
			VideoStreams: []ffmpeg.VideoStream{}, // Empty video streams
		}
		frameRate := getFrameRate(emptyInfo)
		assert.Equal(s.T(), 0.0, frameRate)
	})
}

// TestSaveMediaInfoWithRealFiles tests the saveMediaInfoText function with real sample files.
func (s *MainTestSuite) TestSaveMediaInfoWithRealFiles() {
	// Skip test if not running in the repository root
	resourcesDir := filepath.Join("resources", "test")
	if _, err := os.Stat(resourcesDir); os.IsNotExist(err) {
		s.T().Skip("Skipping test as resources/test directory does not exist")
	}

	// Find sample files in the resources/test directory
	entries, err := os.ReadDir(resourcesDir)
	s.NoError(err)

	// Create a temporary output directory
	outputDir := filepath.Join(s.tempDir, "real_file_output")
	err = os.MkdirAll(outputDir, 0755)
	s.NoError(err)

	// Test each sample file
	for _, entry := range entries {
		// Skip directories
		if entry.IsDir() {
			continue
		}

		// Get file info
		fileName := entry.Name()
		filePath := filepath.Join(resourcesDir, fileName)
		fileInfo, err := os.Stat(filePath)
		s.NoError(err)

		// Skip files larger than 10MB to avoid slowing down tests
		if fileInfo.Size() > 10*1024*1024 {
			s.T().Logf("Skipping large file %s (%.2f MB)", fileName, float64(fileInfo.Size())/(1024*1024))
			continue
		}

		s.Run(fmt.Sprintf("Sample_%s", fileName), func() {
			// Create a fake container info for the sample
			containerInfo := createFakeContainerInfo(filePath, fileInfo.Size())

			// Set title explicitly for testing
			containerInfo.General.Tags["title"] = "Sample: " + fileName

			// Create a nested output directory for this sample
			sampleOutputDir := filepath.Join(outputDir, strings.TrimSuffix(fileName, filepath.Ext(fileName)))
			err = os.MkdirAll(sampleOutputDir, 0755)
			s.NoError(err)

			// Save the media info
			err = saveMediaInfoText(containerInfo, sampleOutputDir, s.prober)
			s.NoError(err)

			// Verify the file was created
			infoFile := filepath.Join(sampleOutputDir, "mediainfo.txt")
			_, err = os.Stat(infoFile)
			s.NoError(err)

			// Verify basic content
			content, err := os.ReadFile(infoFile)
			s.NoError(err)
			s.Contains(string(content), "MEDIA INFORMATION SUMMARY")
			s.Contains(string(content), "Sample: "+fileName)
			s.Contains(string(content), fileName)
		})
	}
}

// createFakeContainerInfo creates a fake container info for testing,
// using the provided file path and size.
func createFakeContainerInfo(filePath string, fileSize int64) *ffmpeg.ContainerInfo {
	// Extract the file extension and create appropriate format
	ext := strings.ToLower(filepath.Ext(filePath))
	format := "Unknown"
	switch ext {
	case ".mkv":
		format = "Matroska"
	case ".avi":
		format = "AVI"
	case ".mp4":
		format = "MPEG-4"
	case ".mov":
		format = "QuickTime"
	}

	// Create a basic container info
	containerInfo := &ffmpeg.ContainerInfo{
		General: ffmpeg.GeneralInfo{
			Format:      format,
			Size:        fmt.Sprintf("%d bytes", fileSize),
			Duration:    "60.0 s",
			DurationF:   60.0,
			BitRate:     fmt.Sprintf("%d b/s", fileSize*8/60), // Simple bitrate calculation
			StreamCount: 3,                                    // Assume 3 streams by default
			Tags: map[string]string{
				"file_path": filePath,
				"title":     filepath.Base(filePath),
			},
		},
		VideoStreams: []ffmpeg.VideoStream{
			{
				Format:             "H.264",
				FormatProfile:      "Main",
				Width:              1280,
				Height:             720,
				DisplayAspectRatio: 1.78,
				FrameRate:          24.0,
				BitRate:            fileSize * 8 / 60 * 8 / 10, // 80% of total bitrate for video
				BitDepth:           8,
				ScanType:           "Progressive",
				ColorSpace:         "YUV",
				Language:           "eng",
				Title:              "Main Video",
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
				Title:         "Main Audio",
			},
		},
		SubtitleStreams: []ffmpeg.SubtitleStream{
			{
				Format:   "SRT",
				Language: "eng",
				Title:    "English",
			},
		},
		ChapterStreams: []ffmpeg.ChapterStream{
			{
				ID:        1,
				StartTime: 0.0,
				EndTime:   30.0,
				Title:     "Chapter 1",
			},
			{
				ID:        2,
				StartTime: 30.0,
				EndTime:   60.0,
				Title:     "Chapter 2",
			},
		},
	}

	return containerInfo
}

// TestMainTestSuite runs the test suite.
func TestMainTestSuite(t *testing.T) {
	suite.Run(t, new(MainTestSuite))
}
