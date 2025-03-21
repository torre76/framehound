// Package ffmpeg provides functionality for detecting and working with FFmpeg.
package ffmpeg

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// ProberTestSuite is a test suite for the Prober type.
// It tests the functionality for probing video files and retrieving metadata.
type ProberTestSuite struct {
	suite.Suite
	ffmpegInfo *FFmpegInfo // FFmpeg information for the test environment
	prober     *Prober     // Prober instance under test
}

// SetupTest sets up the test environment before each test.
// It initializes a mock FFmpegInfo and a Prober instance.
func (suite *ProberTestSuite) SetupTest() {
	suite.ffmpegInfo = &FFmpegInfo{
		Installed: true,
		Path:      "/usr/bin/ffmpeg",
		Version:   "4.2.2",
	}
	var err error
	suite.prober, err = NewProber(suite.ffmpegInfo)
	suite.NoError(err)
}

// TestNewProber tests the NewProber constructor function.
// It verifies that the constructor properly handles various input conditions
// and correctly initializes the Prober.
func (suite *ProberTestSuite) TestNewProber() {
	// Test with valid FFmpegInfo
	prober, err := NewProber(suite.ffmpegInfo)
	suite.NoError(err)
	suite.NotNil(prober)
	suite.Equal("/usr/bin/ffprobe", prober.FFprobePath)

	// Test with nil FFmpegInfo
	prober, err = NewProber(nil)
	suite.Error(err)
	suite.Nil(prober)

	// Test with FFmpegInfo where FFmpeg is not installed
	prober, err = NewProber(&FFmpegInfo{Installed: false})
	suite.Error(err)
	suite.Nil(prober)
}

// TestVideoInfoString tests the String method of VideoInfo.
// It verifies that the method correctly formats the video information
// as a string with different combinations of fields.
func (suite *ProberTestSuite) TestVideoInfoString() {
	// Test cases
	testCases := []struct {
		name     string
		info     *VideoInfo
		expected string
	}{
		{
			name: "Complete info",
			info: &VideoInfo{
				Codec:     "h264",
				Width:     1920,
				Height:    1080,
				FrameRate: 30.0,
				Duration:  10.0,
				FilePath:  "test.mp4",
			},
			expected: "Codec: h264, Resolution: 1920x1080, FPS: 30.000, Duration: 10.000000s",
		},
		{
			name: "Codec only",
			info: &VideoInfo{
				Codec:    "h264",
				FilePath: "test.mp4",
			},
			expected: "Codec: h264",
		},
		{
			name: "Resolution only",
			info: &VideoInfo{
				Width:    1920,
				Height:   1080,
				FilePath: "test.mp4",
			},
			expected: "Resolution: 1920x1080",
		},
		{
			name: "Frame rate only",
			info: &VideoInfo{
				FrameRate: 30.0,
				FilePath:  "test.mp4",
			},
			expected: "FPS: 30.000",
		},
		{
			name: "Duration only",
			info: &VideoInfo{
				Duration: 10.0,
				FilePath: "test.mp4",
			},
			expected: "Duration: 10.000000s",
		},
		{
			name: "Non-integer frame rate",
			info: &VideoInfo{
				FrameRate: 23.976,
				FilePath:  "test_23976.mp4",
			},
			expected: "FPS: 23.976",
		},
	}

	// Run test cases
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := tc.info.String()
			suite.Equal(tc.expected, result)
		})
	}
}

// TestProberSuite runs the Prober test suite.
// This is the entry point for running all Prober tests.
func TestProberSuite(t *testing.T) {
	suite.Run(t, new(ProberTestSuite))
}
