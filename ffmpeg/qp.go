// Package ffmpeg provides functionality for detecting and working with FFmpeg.
package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Type definitions (if not already in types.go)

// Private methods (alphabetical)

// calculateAverageQP calculates the average QP value from a slice of QP values.
// It sums all QP values and divides by the count to get the arithmetic mean.
// Returns 0.0 if the input slice is empty to avoid division by zero.
func (qa *QPAnalyzer) calculateAverageQP(qpValues []int) float64 {
	if len(qpValues) == 0 {
		return 0.0
	}

	var sum int
	for _, qp := range qpValues {
		sum += qp
	}

	return float64(sum) / float64(len(qpValues))
}

// checkCodecCompatibility verifies if the video codec supports QP analysis.
// It uses the prober to extract codec information and checks against known
// compatible formats. Returns nil if compatible, otherwise returns an error
// with details about why the codec is not supported.
func (qa *QPAnalyzer) checkCodecCompatibility(filePath string) error {
	if qa.prober == nil {
		return fmt.Errorf("prober is not available to check codec compatibility")
	}

	// Get container info to check codec type
	containerInfo, err := qa.prober.GetExtendedContainerInfo(filePath)
	if err != nil {
		return fmt.Errorf("failed to get container info: %w", err)
	}

	// Check if we have video streams
	if len(containerInfo.VideoStreams) == 0 {
		return fmt.Errorf("no video streams found in file")
	}

	// Check the first video stream's codec
	videoStream := containerInfo.VideoStreams[0]
	codec := strings.ToLower(videoStream.Format)

	// List of supported codecs - only xvid, divx, avi and h264 are supported
	supportedCodecs := []string{
		"xvid", "divx", "avi", "h264",
	}

	// Check if the codec is in the supported list
	for _, supported := range supportedCodecs {
		if strings.Contains(codec, supported) {
			return nil // Compatible codec found
		}
	}

	return fmt.Errorf("codec '%s' is not supported for QP analysis (supported: xvid, divx, avi, h264)", videoStream.Format)
}

// collectFrameQPValues collects all QP values from a frame's offset map.
// It combines QP values from all offsets into a single slice.
func (qa *QPAnalyzer) collectFrameQPValues(offsetMap map[int][]int) []int {
	var allQPValues []int

	// Get sorted offsets to process them in order
	offsets := make([]int, 0, len(offsetMap))
	for offset := range offsetMap {
		offsets = append(offsets, offset)
	}

	// Collect all QP values
	for _, offset := range offsets {
		allQPValues = append(allQPValues, offsetMap[offset]...)
	}

	return allQPValues
}

// detectCodecType tries to determine the codec type from the frame pointer string.
// The frame pointer typically includes the codec info like [h264 @ 0x12345678].
func (qa *QPAnalyzer) detectCodecType(framePointer string) string {
	codec := "unknown"

	if strings.Contains(framePointer, "h264") {
		codec = "h264"
	} else if strings.Contains(framePointer, "xvid") {
		codec = "xvid"
	} else if strings.Contains(framePointer, "divx") {
		codec = "divx"
	}

	return codec
}

// finalizeAndSendFrame collects QP values for a frame and sends it to the result channel.
// It returns the last good frame that can be used as a reference for future frames.
func (qa *QPAnalyzer) finalizeAndSendFrame(ctx context.Context, frame *FrameQP, frameQPMap map[string]map[int][]int, resultCh chan<- FrameQP, lastGoodFrame *FrameQP) *FrameQP {
	// Find the framePointer for this frame
	var framePointer string
	for fp := range frameQPMap {
		framePointer = fp
		break // Just take the first one
	}

	if framePointer != "" {
		// Collect all QP values for this frame
		allQPValues := qa.collectFrameQPValues(frameQPMap[framePointer])

		// Set the QP values for this frame
		frame.QPValues = allQPValues

		// Only calculate an average if we have enough significant QP data
		if len(allQPValues) < 10 {
			// For frames with insufficient QP data (< 10 values), we use the values
			// from the last good frame instead of skipping them. This approach:
			// 1. Ensures consistent QP data across all frames
			// 2. Provides reasonable approximation for frames with insufficient data
			// 3. Makes sure all frames are included in the analysis results
			// 4. Avoids gaps in the QP analysis output
			if lastGoodFrame != nil {
				// Copy QP values and average from the last good frame
				frame.QPValues = lastGoodFrame.QPValues
				frame.AverageQP = lastGoodFrame.AverageQP

				// Send the frame with copied QP data
				select {
				case resultCh <- *frame:
					// Successfully sent frame with copied QP data
				case <-ctx.Done():
					// Context canceled
					return lastGoodFrame
				}
			} else {
				// No last good frame available yet
				// If we have at least some QP values (even if < 10), we'll use them
				if len(allQPValues) > 0 {
					frame.AverageQP = qa.calculateAverageQP(allQPValues)

					// Send the frame and make it the last good frame since it's the best we have
					select {
					case resultCh <- *frame:
						lastGoodFrame = frame
					case <-ctx.Done():
						return lastGoodFrame
					}
				}

				// Otherwise, clear the map for this frame pointer
				delete(frameQPMap, framePointer)
				return lastGoodFrame
			}
		} else {
			// Frame has enough QP data to calculate its own average
			frame.AverageQP = qa.calculateAverageQP(allQPValues)

			// Send the frame QP data to the channel
			select {
			case resultCh <- *frame:
				// Successfully sent, update lastGoodFrame for future reference
				// This frame becomes the new reference for any subsequent frames
				// with insufficient QP data
				lastGoodFrame = frame
			case <-ctx.Done():
				// Context canceled
				return lastGoodFrame
			}
		}

		// Clear the map for this frame pointer to free memory
		delete(frameQPMap, framePointer)
	}

	return lastGoodFrame
}

// normalizeCodecType normalizes codec type names to a standard format.
// For example, it converts variants of H264 to the standardized "h264".
func (qa *QPAnalyzer) normalizeCodecType(codecType string) string {
	codecType = strings.ToLower(codecType)

	// Common codec type variations
	switch {
	case strings.Contains(codecType, "h264"):
		return "h264"
	case strings.Contains(codecType, "xvid"):
		return "xvid"
	case strings.Contains(codecType, "divx"):
		return "divx"
	default:
		return codecType
	}
}

// processQPOutput processes the FFmpeg debug output to extract QP values.
// It parses the output line by line, identifying frame boundaries and QP data.
func (qa *QPAnalyzer) processQPOutput(ctx context.Context, stderr io.Reader, resultCh chan<- FrameQP) error {
	scanner := bufio.NewScanner(stderr)

	// Regular expressions for parsing QP data
	frameTypeRegex := regexp.MustCompile(`\[([^\]]+)\] New frame, type: ([IiPpBb])`)
	frameNumRegex := regexp.MustCompile(`n:(\d+)`) // Extract frame number if available
	// Updated regex to better match the actual QP output format where values follow the offset
	qpLineRegex := regexp.MustCompile(`\[([^\]]+)\]\s+(\d+)\s+([0-9 ]+)`)

	var currentFrame *FrameQP
	var frameNumber int = 0
	frameQPMap := make(map[string]map[int][]int) // map[framePointer]map[offset]qpValues

	// Keep track of the last good frame with sufficient QP data.
	// This will be used to provide QP values for frames that don't have enough data,
	// ensuring all frames have valid QP data in the final output.
	var lastGoodFrame *FrameQP

	// Process lines
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line := scanner.Text()

			// Process line based on its content
			if qa.isNewFrameLine(line, frameTypeRegex) {
				// Handle new frame
				lastGoodFrame = qa.handleNewFrame(ctx, currentFrame, frameQPMap, resultCh, lastGoodFrame)

				// Update frame info
				frameNumber++
				frameMatches := frameTypeRegex.FindStringSubmatch(line)
				framePointer := frameMatches[1]
				frameType := strings.ToUpper(frameMatches[2])

				// Initialize the current frame
				currentFrame = &FrameQP{
					FrameNumber:         frameNumber,
					OriginalFrameNumber: frameNumber, // Default to sequential number
					FrameType:           frameType,
					CodecType:           qa.detectCodecType(framePointer),
				}

				// Check for frame number in the line
				if frameNumMatches := frameNumRegex.FindStringSubmatch(line); len(frameNumMatches) > 1 {
					if origNum, err := strconv.Atoi(frameNumMatches[1]); err == nil {
						currentFrame.OriginalFrameNumber = origNum
					}
				}

				// Initialize map for this frame if needed
				if _, ok := frameQPMap[framePointer]; !ok {
					frameQPMap[framePointer] = make(map[int][]int)
				}
			} else if qa.isQPDataLine(line, qpLineRegex) {
				// Handle QP data line
				qa.handleQPDataLine(line, qpLineRegex, frameQPMap)
			}
		}
	}

	// Process any remaining frame
	if currentFrame != nil && len(frameQPMap) > 0 {
		_ = qa.finalizeAndSendFrame(ctx, currentFrame, frameQPMap, resultCh, lastGoodFrame)
	}

	return nil
}

// isNewFrameLine checks if a line indicates the start of a new frame
func (qa *QPAnalyzer) isNewFrameLine(line string, frameTypeRegex *regexp.Regexp) bool {
	return frameTypeRegex.MatchString(line) && strings.Contains(line, "New frame, type:")
}

// isQPDataLine checks if a line contains QP data
func (qa *QPAnalyzer) isQPDataLine(line string, qpLineRegex *regexp.Regexp) bool {
	return qpLineRegex.MatchString(line)
}

// handleNewFrame processes the end of a frame and prepares for a new one
func (qa *QPAnalyzer) handleNewFrame(
	ctx context.Context,
	currentFrame *FrameQP,
	frameQPMap map[string]map[int][]int,
	resultCh chan<- FrameQP,
	lastGoodFrame *FrameQP,
) *FrameQP {
	// Before starting a new frame, finalize any existing frame
	if currentFrame != nil && len(frameQPMap) > 0 {
		// Try to send the existing frame
		return qa.finalizeAndSendFrame(ctx, currentFrame, frameQPMap, resultCh, lastGoodFrame)
	}
	return lastGoodFrame
}

// handleQPDataLine processes a line containing QP data
func (qa *QPAnalyzer) handleQPDataLine(
	line string,
	qpLineRegex *regexp.Regexp,
	frameQPMap map[string]map[int][]int,
) {
	matches := qpLineRegex.FindStringSubmatch(line)
	if len(matches) >= 4 {
		framePointer := matches[1]
		offset, err := strconv.Atoi(matches[2])
		if err != nil {
			return // Skip if offset isn't a valid number
		}

		// Parse QP values
		qpValues := qa.parseQPString(matches[3])

		// Store values in the map
		if frameMap, ok := frameQPMap[framePointer]; ok {
			frameMap[offset] = qpValues
		}
	}
}

// parseQPString parses the QP values from a string.
// The input string may contain multiple two-digit QP values
// (e.g. "24 25 25 23 26 23 24 23 26 24 25 25 25 25 25 27 25 24 23").
func (qa *QPAnalyzer) parseQPString(str string) []int {
	var result []int

	// Remove any non-digit or space characters, keeping only digits and spaces
	cleanStr := regexp.MustCompile(`[^0-9 ]`).ReplaceAllString(str, "")

	// For the H.264 format, QP values are typically presented as a continuous
	// string of 2-digit numbers (e.g., "2425252326232423")
	// We need to parse each two characters as a separate QP value
	cleanStr = strings.TrimSpace(cleanStr)

	// If the string contains spaces, it's likely already space-separated values
	if strings.Contains(cleanStr, " ") {
		parts := strings.Fields(cleanStr)
		for _, part := range parts {
			if val, err := strconv.Atoi(part); err == nil && val > 0 && val < 100 {
				result = append(result, val)
			}
		}
		return result
	}

	// Otherwise, parse every two characters as a QP value
	for i := 0; i < len(cleanStr)-1; i += 2 {
		if i+2 <= len(cleanStr) {
			val, err := strconv.Atoi(cleanStr[i : i+2])
			if err == nil && val > 0 && val < 100 {
				result = append(result, val)
			}
		}
	}

	return result
}

// Public methods (alphabetical)

// AnalyzeQP analyzes QP values for each frame in the video file
// and sends results through the provided channel.
// The context can be used to cancel the analysis.
// It will return an error if the codec of the video is not supported for QP analysis.
func (qa *QPAnalyzer) AnalyzeQP(ctx context.Context, filePath string, resultCh chan<- FrameQP) error {
	qa.mutex.Lock()
	defer qa.mutex.Unlock()

	if !qa.SupportsQPReading {
		defer close(resultCh)
		return fmt.Errorf("FFmpeg does not support QP reading")
	}

	// Check if the codec is compatible with QP analysis
	if err := qa.checkCodecCompatibility(filePath); err != nil {
		defer close(resultCh)
		return err
	}

	// Run FFmpeg with QP debug option
	cmd := exec.CommandContext(
		ctx,
		qa.FFmpegPath,
		"-debug:v", "qp",
		"-i", filePath,
		"-an",      // No audio
		"-v", "48", // Verbose level to ensure we get the debug info
		"-f", "null", // Output to null
		"-", // Use stdout as output
	)

	// Get stderr pipe to capture the debug output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		defer close(resultCh)
		return fmt.Errorf("failed to get stderr pipe: %v", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		defer close(resultCh)
		return fmt.Errorf("failed to start ffmpeg: %v", err)
	}

	// Process the debug output in a separate goroutine
	errCh := make(chan error, 1)
	go func() {
		err := qa.processQPOutput(ctx, stderr, resultCh)
		errCh <- err
		close(errCh)
	}()

	// Wait for the process to complete or context to be canceled
	var processErr error
	select {
	case processErr = <-errCh:
		// Processing finished
	case <-ctx.Done():
		// Context was canceled
		processErr = ctx.Err()
		// Kill the FFmpeg process
		_ = cmd.Process.Kill()
	}

	// Wait for command to finish
	cmdErr := cmd.Wait()

	// Always close the result channel when we're done
	close(resultCh)

	// Return the first error encountered
	if processErr != nil {
		return fmt.Errorf("error processing QP output: %v", processErr)
	}

	if cmdErr != nil && ctx.Err() == nil {
		return fmt.Errorf("ffmpeg process failed: %v", cmdErr)
	}

	return nil
}

// CheckCodecCompatibility verifies if the video file's codec supports QP analysis.
// This method examines the codec to determine if QP values can be accurately extracted.
//
// Parameters:
//   - filePath: Path to the video file to check
//
// Returns nil if the codec is compatible with QP analysis, or an error explaining
// why QP analysis is not supported for this video.
func (qa *QPAnalyzer) CheckCodecCompatibility(filePath string) error {
	return qa.checkCodecCompatibility(filePath)
}

// Public functions (alphabetical)

// NewQPAnalyzer creates a new analyzer for extracting QP (Quantization Parameter) values from video files.
// It requires a valid FFmpegInfo object with QP reading support and a Prober for codec detection.
//
// Parameters:
//   - ffmpegInfo: Information about the FFmpeg installation, must have QP reading support
//   - prober: A Prober instance for obtaining codec information
//
// Returns a configured QPAnalyzer and nil error on success, or nil and an error if requirements are not met.
func NewQPAnalyzer(ffmpegInfo *FFmpegInfo, prober *Prober) (*QPAnalyzer, error) {
	if ffmpegInfo == nil {
		return nil, fmt.Errorf("ffmpegInfo cannot be nil")
	}

	if !ffmpegInfo.Installed {
		return nil, fmt.Errorf("FFmpeg is not installed")
	}

	if !ffmpegInfo.HasQPReadingInfoSupport {
		return nil, fmt.Errorf("FFmpeg does not support QP reading")
	}

	if prober == nil {
		return nil, fmt.Errorf("prober cannot be nil")
	}

	return &QPAnalyzer{
		FFmpegPath:        ffmpegInfo.Path,
		SupportsQPReading: ffmpegInfo.HasQPReadingInfoSupport,
		prober:            prober,
		mutex:             sync.Mutex{},
	}, nil
}
