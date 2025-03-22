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

// Private methods (alphabetical)

// calculateAverageCUSize calculates the average CU size from a slice of CU size values.
// It sums all CU sizes and divides by the count to get the arithmetic mean.
// Returns 0.0 if the input slice is empty to avoid division by zero.
func (ca *CUAnalyzer) calculateAverageCUSize(cuSizes []int) float64 {
	if len(cuSizes) == 0 {
		return 0.0
	}

	var sum int
	for _, size := range cuSizes {
		sum += size
	}

	return float64(sum) / float64(len(cuSizes))
}

// checkCodecCompatibility verifies if the video codec supports CU analysis.
// It uses the prober to extract codec information and checks against known
// compatible formats. Returns nil if compatible, otherwise returns an error
// with details about why the codec is not supported.
func (ca *CUAnalyzer) checkCodecCompatibility(filePath string) error {
	if ca.prober == nil {
		return fmt.Errorf("prober is not available to check codec compatibility")
	}

	// Get container info to check codec type
	containerInfo, err := ca.prober.GetExtendedContainerInfo(filePath)
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

	// HEVC is the only supported codec for CU analysis
	if strings.Contains(codec, "hevc") || strings.Contains(codec, "h265") {
		return nil // Compatible codec found
	}

	return fmt.Errorf("codec '%s' is not supported for CU analysis (only HEVC/H.265 is supported)", videoStream.Format)
}

// collectFrameCUData collects all CU data from a frame's offset map.
// It combines CU values from all offsets into a single slice.
func (ca *CUAnalyzer) collectFrameCUValues(offsetMap map[int][]int) []int {
	var allCUValues []int

	// Get sorted offsets to process them in order
	offsets := make([]int, 0, len(offsetMap))
	for offset := range offsetMap {
		offsets = append(offsets, offset)
	}

	// Collect all CU values
	for _, offset := range offsets {
		allCUValues = append(allCUValues, offsetMap[offset]...)
	}

	return allCUValues
}

// finalizeAndSendFrame calculates the final CU statistics for a frame
// and sends it to the result channel.
func (ca *CUAnalyzer) finalizeAndSendFrame(
	ctx context.Context,
	currentFrame *FrameCU,
	frameCUMap map[string]map[int][]int,
	resultCh chan<- FrameCU,
	lastGoodFrame *FrameCU,
) *FrameCU {
	// Ensure we have the frame pointer
	var framePointer string
	for fp := range frameCUMap {
		framePointer = fp
		break
	}

	// If we have no frame pointer, return the last good frame
	if framePointer == "" {
		return lastGoodFrame
	}

	// Get the CU data for this frame
	frameMap, ok := frameCUMap[framePointer]
	if !ok || len(frameMap) == 0 {
		// This frame has no CU data, skip it
		// This can happen if the frame was processed but no CU data was found
		return lastGoodFrame
	}

	// Collect all CU values from this frame
	allCUValues := ca.collectFrameCUValues(frameMap)

	// Check if we have enough data to calculate statistics
	if len(allCUValues) > 0 {
		// Update the frame with CU data
		frame := currentFrame
		frame.CUSizes = allCUValues
		frame.AverageCUSize = ca.calculateAverageCUSize(allCUValues)

		// Send the frame CU data to the channel
		select {
		case resultCh <- *frame:
			// Successfully sent, update lastGoodFrame for future reference
			// This frame becomes the new reference for any subsequent frames
			// with insufficient CU data
			lastGoodFrame = frame
		case <-ctx.Done():
			// Context canceled
			return lastGoodFrame
		}
	}

	// Clear the map for this frame pointer to free memory
	delete(frameCUMap, framePointer)

	return lastGoodFrame
}

// normalizeCodecType normalizes codec type names to a standard format.
// For HEVC/H.265, it standardizes to "hevc".
func (ca *CUAnalyzer) normalizeCodecType(codecType string) string {
	codecType = strings.ToLower(codecType)

	// Common codec type variations
	switch {
	case strings.Contains(codecType, "h265"):
		return "hevc"
	case strings.Contains(codecType, "hevc"):
		return "hevc"
	default:
		return codecType
	}
}

// processCUOutput processes the FFmpeg debug output to extract CU information.
// It parses the output line by line, identifying frame boundaries and CU data.
func (ca *CUAnalyzer) processCUOutput(ctx context.Context, reader io.Reader, resultCh chan<- FrameCU) error {
	scanner := bufio.NewScanner(reader)
	var currentFrame *FrameCU
	frameNumber := 0
	var lastGoodFrame *FrameCU

	// Maps to store CU data by frame pointer and offset
	frameCUMap := make(map[string]map[int][]int)

	// Regular expressions for parsing the output
	frameTypeRegex := regexp.MustCompile(`\[(hevc|h265)\s*@\s*([^\]]+)\]\s*nal_unit_type:\s*(\d+)\(([^)]+)\),\s*nuh_layer_id:\s*(\d+),\s*temporal_id:\s*(\d+)`)
	frameNumRegex := regexp.MustCompile(`Decoded frame with POC\s+(\d+)`)
	cuSizeRegex := regexp.MustCompile(`\[(hevc|h265)\s*@\s*([^\]]+)\]\s*CU\s+size\s+(\d+)x(\d+)\s+pos\s+\((\d+),(\d+)\)\s+type\s+(\d+)`)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			line := scanner.Text()
			var err error
			currentFrame, lastGoodFrame, err = ca.processOutputLine(
				line,
				frameTypeRegex,
				frameNumRegex,
				cuSizeRegex,
				ctx,
				currentFrame,
				lastGoodFrame,
				&frameNumber,
				frameCUMap,
				resultCh,
			)
			if err != nil {
				return err
			}
		}
	}

	// Process any remaining frame
	if currentFrame != nil && len(frameCUMap) > 0 {
		_ = ca.finalizeAndSendFrame(ctx, currentFrame, frameCUMap, resultCh, lastGoodFrame)
	}

	return nil
}

// processOutputLine handles a single line of FFmpeg debug output.
// It identifies the type of line and processes it accordingly.
func (ca *CUAnalyzer) processOutputLine(
	line string,
	frameTypeRegex *regexp.Regexp,
	frameNumRegex *regexp.Regexp,
	cuSizeRegex *regexp.Regexp,
	ctx context.Context,
	currentFrame *FrameCU,
	lastGoodFrame *FrameCU,
	frameNumber *int,
	frameCUMap map[string]map[int][]int,
	resultCh chan<- FrameCU,
) (*FrameCU, *FrameCU, error) {
	// Check if this is a NAL unit line (potentially a new frame)
	if frameTypeMatches := frameTypeRegex.FindStringSubmatch(line); frameTypeMatches != nil {
		return ca.processNALUnitLine(frameTypeMatches, frameNumRegex, line, ctx, currentFrame, lastGoodFrame, frameNumber, frameCUMap, resultCh)
	}

	// Check if this is a CU size line
	if cuMatches := cuSizeRegex.FindStringSubmatch(line); cuMatches != nil {
		ca.processCUSizeLine(cuMatches, frameCUMap)
		return currentFrame, lastGoodFrame, nil
	}

	// Check if this is a frame POC line
	if strings.Contains(line, "Decoded frame with POC") {
		ca.processPOCLine(line, frameNumRegex, currentFrame)
	}

	return currentFrame, lastGoodFrame, nil
}

// processNALUnitLine handles a line containing NAL unit information.
// It detects frame boundaries and creates new frames as needed.
func (ca *CUAnalyzer) processNALUnitLine(
	frameTypeMatches []string,
	frameNumRegex *regexp.Regexp,
	line string,
	ctx context.Context,
	currentFrame *FrameCU,
	lastGoodFrame *FrameCU,
	frameNumber *int,
	frameCUMap map[string]map[int][]int,
	resultCh chan<- FrameCU,
) (*FrameCU, *FrameCU, error) {
	codecType := frameTypeMatches[1]
	framePointer := frameTypeMatches[2]
	nalTypeName := frameTypeMatches[4]

	// Check if this is a new frame (by NAL unit type)
	if !ca.isNewFrameNAL(nalTypeName) {
		return currentFrame, lastGoodFrame, nil
	}

	// Handle new frame
	lastGoodFrame = ca.finalizeAndSendFrame(ctx, currentFrame, frameCUMap, resultCh, lastGoodFrame)

	// Initialize the current frame
	*frameNumber++
	currentFrame = &FrameCU{
		FrameNumber:         *frameNumber,
		OriginalFrameNumber: *frameNumber, // Default to sequential number
		FrameType:           ca.nalTypeToFrameType(nalTypeName),
		CodecType:           ca.normalizeCodecType(codecType),
	}

	// Check for frame number in preceding lines
	if frameNumMatches := frameNumRegex.FindStringSubmatch(line); len(frameNumMatches) > 1 {
		if origNum, err := strconv.Atoi(frameNumMatches[1]); err == nil {
			currentFrame.OriginalFrameNumber = origNum
		}
	}

	// Initialize map for this frame if needed
	if _, ok := frameCUMap[framePointer]; !ok {
		frameCUMap[framePointer] = make(map[int][]int)
	}

	return currentFrame, lastGoodFrame, nil
}

// processCUSizeLine handles a line containing CU size information.
// It extracts the CU size and adds it to the frame's CU data.
func (ca *CUAnalyzer) processCUSizeLine(
	cuMatches []string,
	frameCUMap map[string]map[int][]int,
) {
	framePointer := cuMatches[2]
	width, _ := strconv.Atoi(cuMatches[3])
	height, _ := strconv.Atoi(cuMatches[4])
	posX, _ := strconv.Atoi(cuMatches[5])
	posY, _ := strconv.Atoi(cuMatches[6])
	// cuType not used currently, but could be used for future enhancements
	// cuType, _ := strconv.Atoi(cuMatches[7])

	// Calculate CU size (area)
	cuSize := width * height

	// Store the CU size in the frame map
	// Using position as offset
	offset := posY*1000 + posX // Create a unique key based on position
	if frameMap, ok := frameCUMap[framePointer]; ok {
		if _, ok := frameMap[offset]; !ok {
			frameMap[offset] = make([]int, 0)
		}
		frameMap[offset] = append(frameMap[offset], cuSize)
	}
}

// processPOCLine handles a line containing Picture Order Count information.
// It extracts the POC and updates the current frame's original frame number.
func (ca *CUAnalyzer) processPOCLine(
	line string,
	frameNumRegex *regexp.Regexp,
	currentFrame *FrameCU,
) {
	if frameNumMatches := frameNumRegex.FindStringSubmatch(line); len(frameNumMatches) > 1 {
		// Store this POC number for the next frame
		if origNum, err := strconv.Atoi(frameNumMatches[1]); err == nil {
			if currentFrame != nil {
				currentFrame.OriginalFrameNumber = origNum
			}
		}
	}
}

// isNewFrameNAL checks if a NAL unit type indicates the start of a new frame
func (ca *CUAnalyzer) isNewFrameNAL(nalTypeName string) bool {
	// Frame boundary NAL unit types
	frameNALTypes := map[string]bool{
		"IDR_W_RADL": true, // 19
		"IDR_N_LP":   true, // 20
		"CRA_NUT":    true, // 21
		"TRAIL_N":    true, // 0
		"TRAIL_R":    true, // 1
		"TSA_N":      true, // 2
		"TSA_R":      true, // 3
		"STSA_N":     true, // 4
		"STSA_R":     true, // 5
		"RADL_N":     true, // 6
		"RADL_R":     true, // 7
		"RASL_N":     true, // 8
		"RASL_R":     true, // 9
	}

	return frameNALTypes[nalTypeName]
}

// nalTypeToFrameType converts NAL unit type to frame type (I, P, B)
func (ca *CUAnalyzer) nalTypeToFrameType(nalTypeName string) string {
	// Map NAL unit types to frame types
	switch nalTypeName {
	case "IDR_W_RADL", "IDR_N_LP", "CRA_NUT":
		return "I" // I-frame (Intra)
	case "TRAIL_N", "TRAIL_R", "TSA_N", "TSA_R", "STSA_N", "STSA_R":
		return "P" // P-frame (Predicted)
	case "RADL_N", "RADL_R", "RASL_N", "RASL_R":
		return "B" // B-frame (Bi-directional)
	default:
		return "?" // Unknown
	}
}

// Public methods (alphabetical)

// AnalyzeCU analyzes Coding Unit (CU) sizes for each frame in the HEVC video file
// and sends results through the provided channel.
// The context can be used to cancel the analysis.
// It will return an error if the codec of the video is not HEVC or not supported for CU analysis.
func (ca *CUAnalyzer) AnalyzeCU(ctx context.Context, filePath string, resultCh chan<- FrameCU) error {
	ca.mutex.Lock()
	defer ca.mutex.Unlock()

	if !ca.SupportsCUReading {
		defer close(resultCh)
		return fmt.Errorf("FFmpeg does not support CU reading for HEVC")
	}

	// Check if the codec is compatible with CU analysis
	if err := ca.checkCodecCompatibility(filePath); err != nil {
		defer close(resultCh)
		return err
	}

	// Run FFmpeg with debug:v qp option (which provides CU info for HEVC)
	cmd := exec.CommandContext(
		ctx,
		ca.FFmpegPath,
		"-debug:v", "qp", // Use qp debug flag which works for HEVC CU info
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
		err := ca.processCUOutput(ctx, stderr, resultCh)
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
		return fmt.Errorf("error processing CU output: %v", processErr)
	}

	if cmdErr != nil && ctx.Err() == nil {
		return fmt.Errorf("ffmpeg process failed: %v", cmdErr)
	}

	return nil
}

// CheckCodecCompatibility verifies if the video file's codec supports CU analysis.
// This method examines the codec to determine if CU sizes can be accurately extracted.
//
// Parameters:
//   - filePath: Path to the video file to check
//
// Returns nil if the codec is compatible with CU analysis, or an error explaining
// why CU analysis is not supported for this video.
func (ca *CUAnalyzer) CheckCodecCompatibility(filePath string) error {
	return ca.checkCodecCompatibility(filePath)
}

// Public functions (alphabetical)

// NewCUAnalyzer creates a new analyzer for extracting CU (Coding Unit) information from HEVC video files.
// It requires a valid FFmpegInfo object with CU reading support and a Prober for codec detection.
//
// Parameters:
//   - ffmpegInfo: Information about the FFmpeg installation, must have CU reading support
//   - prober: A Prober instance for obtaining codec information
//
// Returns a configured CUAnalyzer and nil error on success, or nil and an error if requirements are not met.
func NewCUAnalyzer(ffmpegInfo *FFmpegInfo, prober *Prober) (*CUAnalyzer, error) {
	if ffmpegInfo == nil {
		return nil, fmt.Errorf("ffmpegInfo cannot be nil")
	}

	if !ffmpegInfo.Installed {
		return nil, fmt.Errorf("FFmpeg is not installed")
	}

	if !ffmpegInfo.HasCUReadingInfoSupport {
		return nil, fmt.Errorf("FFmpeg does not support CU reading for HEVC")
	}

	if prober == nil {
		return nil, fmt.Errorf("prober cannot be nil")
	}

	return &CUAnalyzer{
		FFmpegPath:        ffmpegInfo.Path,
		SupportsCUReading: ffmpegInfo.HasCUReadingInfoSupport,
		prober:            prober,
		mutex:             sync.Mutex{},
	}, nil
}
