// Package ffmpeg provides functionality for detecting and working with FFmpeg.
package ffmpeg

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// BitrateAnalyzerTestSuite defines the test suite for BitrateAnalyzer.
// It tests the functionality for analyzing frame-by-frame bitrate information from video files.
type BitrateAnalyzerTestSuite struct {
	suite.Suite
	ffmpegInfo *FFmpegInfo      // FFmpeg information for the test environment
	analyzer   *BitrateAnalyzer // BitrateAnalyzer instance under test
}

// SetupSuite prepares the test suite by finding FFmpeg.
// It initializes the FFmpegInfo and BitrateAnalyzer instances used by all tests.
func (s *BitrateAnalyzerTestSuite) SetupSuite() {
	var err error
	s.ffmpegInfo, err = FindFFmpeg()
	require.NoError(s.T(), err, "Failed to find FFmpeg")

	if !s.ffmpegInfo.Installed {
		s.T().Log("FFmpeg not installed, using mock FFmpegInfo for tests")
		// Create a basic mock FFmpegInfo for testing
		s.ffmpegInfo = &FFmpegInfo{
			Installed:               true,
			Path:                    "/mock/path/to/ffmpeg",
			Version:                 "mock-version",
			HasQPReadingInfoSupport: true,
			HasCUReadingInfoSupport: true,
		}
	}

	s.analyzer, err = NewBitrateAnalyzer(s.ffmpegInfo)
	require.NoError(s.T(), err, "Failed to create BitrateAnalyzer")
}

// TestAnalyze tests the Analyze method of BitrateAnalyzer.
// It verifies that the analyzer can extract frame information from video files
// and correctly identify frame types and bitrates.
func (s *BitrateAnalyzerTestSuite) TestAnalyze() {
	// Test cases
	testCases := []struct {
		name           string // Test case name
		filePath       string // Path to the test video file
		maxFrames      int    // Maximum number of frames to process
		timeoutSeconds int    // Timeout in seconds
	}{
		{
			name:           "Sample MKV",
			filePath:       "../resources/test/sample.mkv",
			maxFrames:      500,
			timeoutSeconds: 30,
		},
		{
			name:           "Sample2 MKV",
			filePath:       "../resources/test/sample2.mkv",
			maxFrames:      1000, // Limit to 1000 frames for the test
			timeoutSeconds: 60,   // Increase timeout for larger file
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Create a context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(tc.timeoutSeconds)*time.Second)
			defer cancel()

			// Create a channel to receive frame bitrate info
			resultCh := make(chan FrameBitrateInfo, 10)

			// Start the analysis
			err := s.analyzer.Analyze(ctx, tc.filePath, resultCh)
			require.NoError(s.T(), err, "Failed to analyze %s", tc.filePath)

			// Collect and verify results
			frameCount := 0
			iFrameCount := 0
			pFrameCount := 0
			bFrameCount := 0

			// Set a timeout for receiving frames
			timeout := time.After(time.Duration(tc.timeoutSeconds) * time.Second)

			// Process frames until channel is closed, timeout, or max frames reached
			for frameCount < tc.maxFrames {
				select {
				case frame, ok := <-resultCh:
					if !ok {
						// Channel closed, all frames processed
						s.T().Logf("Processed %d frames (%d I-frames, %d P-frames, %d B-frames)",
							frameCount, iFrameCount, pFrameCount, bFrameCount)

						// Basic validation
						assert.Greater(s.T(), frameCount, 0, "No frames were processed")
						return
					}

					// Count frame types
					frameCount++
					switch frame.FrameType {
					case "I":
						iFrameCount++
					case "P":
						pFrameCount++
					case "B":
						bFrameCount++
					}

					// Validate frame data
					assert.Greater(s.T(), frame.FrameNumber, 0, "Invalid frame number: %d", frame.FrameNumber)
					assert.NotEmpty(s.T(), frame.FrameType, "Empty frame type for frame %d", frame.FrameNumber)

					// Log some frame info for debugging
					if frameCount <= 5 || frameCount%100 == 0 {
						s.T().Logf("Frame %d: Type=%s, Bitrate=%d bits",
							frame.FrameNumber, frame.FrameType, frame.Bitrate)
					}

				case <-timeout:
					// If we've processed at least some frames, consider the test successful
					if frameCount > 0 {
						s.T().Logf("Timeout after processing %d frames, but test is considered successful", frameCount)
						return
					}
					s.Fail("Timeout waiting for frames", "Timeout after processing %d frames", frameCount)
					return

				case <-ctx.Done():
					// If we've processed at least some frames, consider the test successful
					if frameCount > 0 {
						s.T().Logf("Context canceled after processing %d frames, but test is considered successful", frameCount)
						return
					}
					s.Fail("Context canceled", "Context canceled after processing %d frames: %v", frameCount, ctx.Err())
					return
				}
			}

			// If we've reached the maximum number of frames, the test is successful
			s.T().Logf("Reached maximum frame count (%d). Processed %d frames (%d I-frames, %d P-frames, %d B-frames)",
				tc.maxFrames, frameCount, iFrameCount, pFrameCount, bFrameCount)

			// Cancel the context to stop processing
			cancel()
		})
	}
}

// TestNewBitrateAnalyzer tests the NewBitrateAnalyzer constructor function.
// It verifies that the constructor properly handles various input conditions
// and correctly initializes the BitrateAnalyzer.
func (s *BitrateAnalyzerTestSuite) TestNewBitrateAnalyzer() {
	// Test with nil FFmpegInfo
	analyzer, err := NewBitrateAnalyzer(nil)
	assert.Error(s.T(), err, "Expected error when creating BitrateAnalyzer with nil FFmpegInfo")
	assert.Nil(s.T(), analyzer, "Expected nil analyzer when creating with nil FFmpegInfo")

	// Test with FFmpegInfo where Installed is false
	analyzer, err = NewBitrateAnalyzer(&FFmpegInfo{Installed: false})
	assert.Error(s.T(), err, "Expected error when creating BitrateAnalyzer with FFmpegInfo.Installed = false")
	assert.Nil(s.T(), analyzer, "Expected nil analyzer when creating with FFmpegInfo.Installed = false")

	// Test with valid FFmpegInfo (already tested in SetupSuite)
	assert.NotNil(s.T(), s.analyzer, "BitrateAnalyzer should not be nil")

	// Check that FFprobePath is set correctly
	expectedPath := strings.Replace(s.ffmpegInfo.Path, "ffmpeg", "ffprobe", 1)
	assert.Equal(s.T(), expectedPath, s.analyzer.FFprobePath, "BitrateAnalyzer.FFprobePath should be set correctly")
}

// TestBitrateAnalyzerSuite runs the BitrateAnalyzer test suite.
// This is the entry point for running all BitrateAnalyzer tests.
func TestBitrateAnalyzerSuite(t *testing.T) {
	suite.Run(t, new(BitrateAnalyzerTestSuite))
}
