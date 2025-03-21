// Package ffmpeg provides functionality for detecting and working with FFmpeg.
package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Private methods (alphabetical)
// None

// Public methods (alphabetical)

// Analyze analyzes the bitrate of each frame in the video file and sends the results
// through the provided channel. It returns an error if the analysis fails.
// The context can be used to cancel the analysis.
//
// The function processes frames one by one and sends FrameBitrateInfo objects through
// the resultCh channel. The caller should read from this channel until it's closed
// or the context is canceled.
//
// This method is designed to handle large video files efficiently by streaming
// the results rather than collecting them all in memory.
func (b *BitrateAnalyzer) Analyze(ctx context.Context, filePath string, resultCh chan<- FrameBitrateInfo) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Create the FFprobe command to extract frame information
	cmd := exec.CommandContext(
		ctx,
		b.FFprobePath,
		"-v", "quiet",
		"-select_streams", "v:0", // Select first video stream
		"-show_frames",          // Show frame information
		"-print_format", "json", // Output in JSON format
		filePath,
	)

	// Get stdout pipe
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFprobe: %w", err)
	}

	// Start a goroutine to process frames
	go func() {
		defer close(resultCh)

		// Create a JSON decoder
		decoder := json.NewDecoder(stdout)

		// Read the opening object
		_, err := decoder.Token()
		if err != nil {
			// Error handling, close channel and return
			return
		}

		// Read "frames" key
		t, err := decoder.Token()
		if err != nil {
			// Error handling, close channel and return
			return
		}
		if t != "frames" {
			// Expected 'frames' key but got something else
			return
		}

		// Read opening array bracket
		_, err = decoder.Token()
		if err != nil {
			// Error handling, close channel and return
			return
		}

		// Process each frame
		frameNumber := 1 // Start from 1 instead of 0

		for decoder.More() {
			// Check if the context is canceled
			if ctx.Err() != nil {
				return
			}

			// Decode frame info
			var frameInfo ffprobeFrameInfo
			if err := decoder.Decode(&frameInfo); err != nil {
				// Skip this frame and continue with the next one
				continue
			}

			// Skip non-video frames
			if frameInfo.MediaType != "video" {
				continue
			}

			// Calculate bitrate from pkt_size (which is in bits)
			bitrate, _ := frameInfo.PktSize.Int64()
			bitrate *= 8 // Convert bytes to bits

			// Extract PTS and DTS
			pts, _ := frameInfo.PktPts.Int64()
			dts, _ := frameInfo.PktDts.Int64()

			// Create a FrameBitrateInfo
			frameBI := FrameBitrateInfo{
				FrameNumber: frameNumber,
				FrameType:   frameInfo.PictType,
				Bitrate:     bitrate,
				PTS:         pts,
				DTS:         dts,
			}

			// Send to the channel
			select {
			case resultCh <- frameBI:
				// Frame sent successfully
			case <-ctx.Done():
				// Context was canceled
				return
			}

			frameNumber++
		}

		// Wait for the command to finish
		_ = cmd.Wait()
	}()

	return nil
}

// Public functions (alphabetical)

// NewBitrateAnalyzer creates a new BitrateAnalyzer instance.
// It requires a valid FFmpegInfo object with FFmpeg installed.
// Returns an error if FFmpeg is not installed or the info is nil.
func NewBitrateAnalyzer(ffmpegInfo *FFmpegInfo) (*BitrateAnalyzer, error) {
	if ffmpegInfo == nil || !ffmpegInfo.Installed {
		return nil, fmt.Errorf("FFmpeg is not installed")
	}

	// Replace ffmpeg with ffprobe in the path
	ffprobePath := strings.Replace(ffmpegInfo.Path, "ffmpeg", "ffprobe", 1)

	return &BitrateAnalyzer{
		FFprobePath: ffprobePath,
	}, nil
}
