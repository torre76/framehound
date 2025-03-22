// Package ffmpeg provides functionality for detecting and working with FFmpeg.
// It offers tools for analyzing video files, extracting media information,
// and processing frame-level data.
package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Private constants (alphabetical)
// None currently defined

// Public constants (alphabetical)
// None currently defined

// Private variables (alphabetical)
// None currently defined

// Public variables (alphabetical)
// None currently defined

// Private functions (alphabetical)
// None currently defined

// Public functions (alphabetical)

// NewBitrateAnalyzer creates a new BitrateAnalyzer instance with the provided FFmpeg information.
// It validates that FFmpeg is available and properly installed on the system before
// creating the analyzer. If FFmpeg is not available, an error is returned.
func NewBitrateAnalyzer(ffmpegInfo *FFmpegInfo) (*BitrateAnalyzer, error) {
	if ffmpegInfo == nil || !ffmpegInfo.Installed {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	// Derive ffprobe path from ffmpeg path
	ffprobePath := strings.Replace(ffmpegInfo.Path, "ffmpeg", "ffprobe", 1)

	return &BitrateAnalyzer{
		FFprobePath: ffprobePath,
	}, nil
}

// Private methods (alphabetical)
// None currently defined

// Public methods (alphabetical)

// Analyze processes a video file to extract frame-by-frame bitrate information.
// It examines each video frame, calculating its bitrate and collecting metadata such as
// frame type and timestamps. Results are streamed through the provided channel to
// efficiently handle large video files without excessive memory usage.
//
// The context parameter allows for cancellation of long-running operations.
// The filePath parameter specifies the video file to analyze.
// The resultCh channel receives FrameBitrateInfo objects for each video frame.
//
// This method is thread-safe and can be called concurrently for different files.
func (b *BitrateAnalyzer) Analyze(ctx context.Context, filePath string, resultCh chan<- FrameBitrateInfo) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Create a child context with cancellation so we can stop all operations
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up command and start it
	cmd, stdout, err := b.setupCommand(childCtx, filePath)
	if err != nil {
		return err
	}

	// Set up done channel and error for processing
	done := make(chan struct{})
	var processErr error

	// Process the data in a goroutine
	go func() {
		defer close(done)
		processErr = b.processOutput(childCtx, stdout, resultCh, cancel)
	}()

	return b.waitForCompletion(ctx, cmd, done, processErr, cancel)
}

// setupCommand creates and starts the FFprobe command.
func (b *BitrateAnalyzer) setupCommand(ctx context.Context, filePath string) (*exec.Cmd, io.ReadCloser, error) {
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
		return nil, nil, fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start FFprobe: %w", err)
	}

	return cmd, stdout, nil
}

// processOutput handles the JSON output from FFprobe and extracts frame information.
func (b *BitrateAnalyzer) processOutput(ctx context.Context, stdout io.ReadCloser, resultCh chan<- FrameBitrateInfo, cancel context.CancelFunc) error {
	// Create a decoder for streaming JSON
	decoder := json.NewDecoder(stdout)

	// Parse the initial JSON structure
	if err := b.parseJSONHeader(decoder, cancel); err != nil {
		return err
	}

	// Process frames
	return b.processFrames(ctx, decoder, resultCh, cancel)
}

// parseJSONHeader handles the JSON structure before the frame array.
func (b *BitrateAnalyzer) parseJSONHeader(decoder *json.Decoder, cancel context.CancelFunc) error {
	// Look for the start of frames array
	_, err := decoder.Token() // We don't use the token value
	if err != nil {
		cancel() // Cancel the command
		return fmt.Errorf("error parsing JSON token: %w", err)
	}

	fieldName, err := decoder.Token()
	if err != nil {
		cancel() // Cancel the command
		return fmt.Errorf("error parsing JSON field name: %w", err)
	}

	if fieldName != "frames" {
		cancel() // Cancel the command
		return fmt.Errorf("unexpected JSON field: %v, expected 'frames'", fieldName)
	}

	// Expect array start
	_, err = decoder.Token() // We don't use the array token value
	if err != nil {
		cancel() // Cancel the command
		return fmt.Errorf("error parsing JSON array start: %w", err)
	}

	return nil
}

// processFrames handles the individual frame data from the JSON.
func (b *BitrateAnalyzer) processFrames(ctx context.Context, decoder *json.Decoder, resultCh chan<- FrameBitrateInfo, cancel context.CancelFunc) error {
	frameNumber := 0

	for decoder.More() {
		// Check if context is canceled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue processing
		}

		// Decode frame info
		var frameInfo ffprobeFrameInfo
		if err := decoder.Decode(&frameInfo); err != nil {
			cancel() // Cancel the command
			return fmt.Errorf("error decoding frame info: %w", err)
		}

		// Skip non-video frames
		if frameInfo.MediaType != "video" {
			continue
		}

		// Create and send frame info
		if err := b.processVideoFrame(ctx, frameInfo, frameNumber, resultCh); err != nil {
			return err
		}

		frameNumber++
	}

	return nil
}

// processVideoFrame extracts information from a video frame and sends it to the result channel.
func (b *BitrateAnalyzer) processVideoFrame(ctx context.Context, frameInfo ffprobeFrameInfo, frameNumber int, resultCh chan<- FrameBitrateInfo) error {
	// Extract frame size in bits
	pktSize, err := frameInfo.PktSize.Int64()
	if err != nil {
		return nil // Skip frames with invalid size, not a fatal error
	}

	// Extract PTS (Presentation Timestamp)
	pts, _ := frameInfo.PktPts.Int64()

	// Extract DTS (Decoding Timestamp)
	dts, _ := frameInfo.PktDts.Int64()

	// Determine frame type from picture type
	frameType := strings.ToUpper(frameInfo.PictType)
	if frameType == "" {
		frameType = "?"
	}

	// Create frame bitrate info
	info := FrameBitrateInfo{
		FrameNumber: frameNumber,
		FrameType:   frameType,
		Bitrate:     pktSize * 8, // Convert bytes to bits
		PTS:         pts,
		DTS:         dts,
	}

	// Send to the channel
	select {
	case <-ctx.Done():
		return ctx.Err()
	case resultCh <- info:
		// Successfully sent
	}

	return nil
}

// waitForCompletion waits for the processing to complete or the context to be cancelled.
func (b *BitrateAnalyzer) waitForCompletion(ctx context.Context, cmd *exec.Cmd, done chan struct{}, processErr error, cancel context.CancelFunc) error {
	// Wait for completion or timeout
	select {
	case <-done:
		// Normal completion
		if processErr != nil {
			return processErr
		}
		err := cmd.Wait()
		if err != nil && ctx.Err() == nil { // Only return command error if not due to cancellation
			return fmt.Errorf("ffprobe command failed: %w", err)
		}
		return ctx.Err() // Return context error if present, nil otherwise
	case <-ctx.Done():
		// Context cancelled or timed out
		cancel() // Make sure child context is cancelled
		<-done   // Wait for processing goroutine to complete
		return ctx.Err()
	}
}
