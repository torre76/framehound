// Package ffmpeg provides functionality for detecting and working with FFmpeg,
// including tools for analyzing video coding units, extracting media information,
// and processing frame-level data.
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
)

// Private methods (alphabetical)

// calculateAverageCUSize calculates the average Coding Unit size from a slice of CU size values.
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

	// Check if any video stream has a compatible codec
	for _, stream := range containerInfo.VideoStreams {
		if ca.isCompatibleCodec(stream.Format) {
			return nil
		}
	}

	// No compatible codec found
	return fmt.Errorf("no compatible video codec found for CU analysis. Only H.265/HEVC and AV1 are supported")
}

// collectFrameCUValues aggregates CU values from the offset map into a single slice.
// It takes a map of block offsets to CU values and flattens it into a single array
// of all CU values, allowing for statistical analysis of the entire frame's CU distribution.
func (ca *CUAnalyzer) collectFrameCUValues(offsetMap map[int][]int) []int {
	var allCUValues []int
	for _, cuValues := range offsetMap {
		allCUValues = append(allCUValues, cuValues...)
	}
	return allCUValues
}

// isCompatibleCodec determines if a codec is supported for CU analysis.
// It checks if the provided codec identifier matches one of the supported
// formats (HEVC/H.265 or AV1). These are the codecs for which
// FFmpeg provides CU debugging information.
func (ca *CUAnalyzer) isCompatibleCodec(codec string) bool {
	lowerCodec := strings.ToLower(codec)
	return lowerCodec == "hevc" ||
		lowerCodec == "h265" ||
		lowerCodec == "av1"
}

// finalizeAndSendFrame completes processing of the current frame and sends it to the result channel.
// It calculates final statistics for the frame, ensures all data is properly set,
// sends the frame to the result channel if valid, and returns the frame as the last good frame
// for reference in case of future errors. This helps maintain continuous data flow even
// when some frames have parsing issues.
func (ca *CUAnalyzer) finalizeAndSendFrame(
	ctx context.Context,
	currentFrame *FrameCU,
	frameCUMap map[string]map[int][]int,
	resultCh chan<- FrameCU,
	lastGoodFrame *FrameCU,
) *FrameCU {
	if currentFrame == nil || currentFrame.FrameNumber <= 0 {
		return lastGoodFrame
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return lastGoodFrame
	default:
		// Continue processing
	}

	// Get codec type
	codecType := ca.normalizeCodecType(currentFrame.CodecType)
	if codecType == "" {
		// Try to infer codec type from the last good frame
		if lastGoodFrame != nil && lastGoodFrame.CodecType != "" {
			codecType = ca.normalizeCodecType(lastGoodFrame.CodecType)
		}
	}

	// Process CU data from appropriate map
	if codecType != "" && frameCUMap[codecType] != nil {
		// Collect all CU values from the appropriate codec map
		allCUValues := ca.collectFrameCUValues(frameCUMap[codecType])
		currentFrame.CUSizes = allCUValues
		currentFrame.AverageCUSize = ca.calculateAverageCUSize(allCUValues)

		// Clear data for this codec type after processing
		frameCUMap[codecType] = make(map[int][]int)
	}

	// Send frame data to channel if we have valid data
	if currentFrame.FrameNumber > 0 && len(currentFrame.CUSizes) > 0 {
		select {
		case resultCh <- *currentFrame:
			// Frame sent successfully
		case <-ctx.Done():
			// Context was cancelled, stop processing
			return lastGoodFrame
		}
		return currentFrame
	}

	return lastGoodFrame
}

// normalizeCodecType standardizes codec type strings to consistent identifiers.
// It converts various codec name formats to standard lowercase identifiers,
// ensuring consistent processing regardless of how FFmpeg reports the codec.
// Handles common variations in codec naming conventions.
func (ca *CUAnalyzer) normalizeCodecType(codecType string) string {
	// Normalize to lowercase
	lowerType := strings.ToLower(codecType)

	// Map to standard names
	if strings.Contains(lowerType, "h265") || strings.Contains(lowerType, "hevc") {
		return "hevc"
	} else if strings.Contains(lowerType, "av1") {
		return "av1"
	}
	return lowerType
}

// processCUOutput parses FFmpeg's debug output to extract CU values and frame information.
// It processes the stderr output from FFmpeg, identifying frame boundaries and CU values,
// builds frame objects with their associated CU data, and streams the results through
// the provided channel. This is the core parsing logic for CU analysis.
func (ca *CUAnalyzer) processCUOutput(ctx context.Context, reader io.Reader, resultCh chan<- FrameCU) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024) // 10MB max buffer size

	// Regular expressions for parsing CU data
	frameTypeRegex := regexp.MustCompile(`New NAL unit \(\d+ bytes\) of type (\w+)`)
	frameNumRegex := regexp.MustCompile(`POC: (\d+)`)
	cuSizeRegex := regexp.MustCompile(`CU at (\d+) (\d+) coded as (\w+) \((\d+)x(\d+)\)`)

	// Maps to store CU values by codec type and block offset
	frameCUMap := map[string]map[int][]int{
		"hevc": make(map[int][]int),
		"av1":  make(map[int][]int),
	}

	var currentFrame *FrameCU
	var lastGoodFrame *FrameCU
	var frameNumber int = 0
	var err error

	// Process each line from FFmpeg output
	for scanner.Scan() {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue processing
		}

		line := scanner.Text()
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

	// Process any remaining frame
	if currentFrame != nil {
		ca.finalizeAndSendFrame(ctx, currentFrame, frameCUMap, resultCh, lastGoodFrame)
	}

	return nil
}

// processOutputLine analyzes a single line of FFmpeg debug output to extract frame or CU information.
// It identifies the type of line (NAL unit, POC, or CU size) and delegates to the appropriate
// handler function. This maintains clean separation of concerns in the parsing logic.
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
	// Check if line contains NAL unit info (new frame)
	if frameTypeMatches := frameTypeRegex.FindStringSubmatch(line); len(frameTypeMatches) > 1 {
		return ca.processNALUnitLine(
			frameTypeMatches,
			line,
			ctx,
			currentFrame,
			lastGoodFrame,
			frameNumber,
			frameCUMap,
			resultCh,
		)
	}

	// Check if line contains POC info (picture order count)
	if strings.Contains(line, "POC:") && currentFrame != nil {
		ca.processPOCLine(line, frameNumRegex, currentFrame)
	}

	// Check if line contains CU size info
	if cuMatches := cuSizeRegex.FindStringSubmatch(line); len(cuMatches) > 5 {
		ca.processCUSizeLine(cuMatches, frameCUMap)
	}

	return currentFrame, lastGoodFrame, nil
}

// processNALUnitLine handles lines containing Network Abstraction Layer unit information.
// It detects the start of a new frame based on the NAL unit type, finalizes the current frame
// if one exists, and initializes a new frame structure. This is crucial for properly
// segmenting the CU data between frames.
func (ca *CUAnalyzer) processNALUnitLine(
	frameTypeMatches []string,
	line string,
	ctx context.Context,
	currentFrame *FrameCU,
	lastGoodFrame *FrameCU,
	frameNumber *int,
	frameCUMap map[string]map[int][]int,
	resultCh chan<- FrameCU,
) (*FrameCU, *FrameCU, error) {
	nalTypeName := frameTypeMatches[1]

	// Check if this NAL unit represents a new frame
	if ca.isNewFrameNAL(nalTypeName) {
		// Process the previous frame before starting a new one
		if currentFrame != nil {
			lastGoodFrame = ca.finalizeAndSendFrame(ctx, currentFrame, frameCUMap, resultCh, lastGoodFrame)
		}

		// Increment frame number and create a new frame
		*frameNumber++
		frameType := ca.nalTypeToFrameType(nalTypeName)

		// Determine codec type from the line
		codecType := "hevc" // Default to HEVC
		if strings.Contains(line, "av1") {
			codecType = "av1"
		}

		// Initialize the current frame
		currentFrame = &FrameCU{
			FrameNumber:         *frameNumber,
			OriginalFrameNumber: *frameNumber, // Will be updated if POC is found
			FrameType:           frameType,
			CodecType:           codecType,
		}

		// Initialize map for this codec if needed
		if _, ok := frameCUMap[codecType]; !ok {
			frameCUMap[codecType] = make(map[int][]int)
		}
	}

	return currentFrame, lastGoodFrame, nil
}

// processCUSizeLine extracts coding unit size information from a matched line.
// It parses the CU position and dimensions, calculates the size (width × height),
// and stores it in the appropriate codec's map. This builds up the CU size distribution
// data for each frame.
func (ca *CUAnalyzer) processCUSizeLine(
	cuMatches []string,
	frameCUMap map[string]map[int][]int,
) {
	// Parse CU position and size
	xPos, _ := strconv.Atoi(cuMatches[1])
	yPos, _ := strconv.Atoi(cuMatches[2])
	width, _ := strconv.Atoi(cuMatches[4])
	height, _ := strconv.Atoi(cuMatches[5])

	// Calculate CU size (area)
	cuSize := width * height

	// Create a unique offset based on position
	offset := xPos*1000 + yPos

	// Detect the codec type based on the sizes (heuristic)
	var codecType string
	if width <= 64 && height <= 64 {
		codecType = "hevc" // H.265/HEVC typically uses CUs up to 64x64
	} else {
		codecType = "av1" // AV1 can use larger CUs
	}

	// Store the CU size in the appropriate map
	if frameCUMap[codecType] != nil {
		frameCUMap[codecType][offset] = append(frameCUMap[codecType][offset], cuSize)
	}
}

// processPOCLine extracts the Picture Order Count from a line of FFmpeg debug output.
// It updates the frame's original frame number with the POC value, which represents
// the actual display order of the frame in the video sequence.
func (ca *CUAnalyzer) processPOCLine(
	line string,
	frameNumRegex *regexp.Regexp,
	currentFrame *FrameCU,
) {
	if matches := frameNumRegex.FindStringSubmatch(line); len(matches) > 1 {
		if pocNum, err := strconv.Atoi(matches[1]); err == nil {
			currentFrame.OriginalFrameNumber = pocNum
		}
	}
}

// isNewFrameNAL determines if a NAL unit type indicates the start of a new frame.
// It checks the NAL unit type against known types that mark frame boundaries
// in H.265/HEVC and AV1 bitstreams.
func (ca *CUAnalyzer) isNewFrameNAL(nalTypeName string) bool {
	// NAL unit types that indicate a new frame in HEVC
	newFrameNALTypes := map[string]bool{
		"IDR_W_RADL":         true,
		"IDR_N_LP":           true,
		"CRA_NUT":            true,
		"TRAIL_R":            true,
		"TRAIL_N":            true,
		"TSA_N":              true,
		"TSA_R":              true,
		"STSA_N":             true,
		"STSA_R":             true,
		"BLA_W_LP":           true,
		"BLA_W_RADL":         true,
		"BLA_N_LP":           true,
		"RADL_N":             true,
		"RADL_R":             true,
		"RASL_N":             true,
		"RASL_R":             true,
		"OPI_NUT":            true, // AV1
		"TEMPORAL_DELIMITER": true, // AV1
		"FRAME_HEADER":       true, // AV1
		"FRAME":              true, // AV1
	}

	return newFrameNALTypes[nalTypeName]
}

// nalTypeToFrameType converts a NAL unit type name to a simplified frame type (I, P, or B).
// It maps the complex NAL unit type names to the three main frame types based on
// the coding characteristics of each NAL type.
func (ca *CUAnalyzer) nalTypeToFrameType(nalTypeName string) string {
	// Map NAL unit types to frame types
	nalToFrameType := map[string]string{
		"IDR_W_RADL":         "I",
		"IDR_N_LP":           "I",
		"CRA_NUT":            "I",
		"BLA_W_LP":           "I",
		"BLA_W_RADL":         "I",
		"BLA_N_LP":           "I",
		"TRAIL_R":            "P",
		"TRAIL_N":            "P",
		"TSA_N":              "P",
		"TSA_R":              "P",
		"STSA_N":             "P",
		"STSA_R":             "P",
		"RADL_N":             "B",
		"RADL_R":             "B",
		"RASL_N":             "B",
		"RASL_R":             "B",
		"OPI_NUT":            "I", // AV1 - Operating Point Info
		"TEMPORAL_DELIMITER": "I", // AV1 - Temporal Unit Delimiter
		"FRAME_HEADER":       "P", // AV1 - Frame Header
		"FRAME":              "P", // AV1 - Frame
	}

	if frameType, ok := nalToFrameType[nalTypeName]; ok {
		return frameType
	}
	return "P" // Default to P-frame if unknown
}

// Public methods (alphabetical)

// AnalyzeCU extracts frame-by-frame coding unit data from a video file.
// It runs FFmpeg with special debug flags to capture CU sizes for each frame,
// processes the output to extract frame type, number, and CU size distribution,
// and streams the results through the provided channel. This allows for detailed
// analysis of encoding patterns throughout the video file.
func (ca *CUAnalyzer) AnalyzeCU(ctx context.Context, filePath string, resultCh chan<- FrameCU) error {
	// Verify FFmpeg is available
	if ca.FFmpegPath == "" {
		return fmt.Errorf("ffmpeg not available for CU analysis")
	}

	// Check codec compatibility
	if err := ca.checkCodecCompatibility(filePath); err != nil {
		return err
	}

	// Build the FFmpeg command with the appropriate debug options
	cmd := exec.CommandContext(
		ctx,
		ca.FFmpegPath,
		"-hide_banner",
		"-loglevel", "debug",
		"-i", filePath,
		"-f", "null",
		"-",
	)

	// Create a pipe for stderr output where FFmpeg writes debug info
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Process the output
	processErr := ca.processCUOutput(ctx, stderr, resultCh)

	// Wait for the command to complete
	cmdErr := cmd.Wait()

	// Return any error that occurred during processing or command execution
	if processErr != nil {
		return fmt.Errorf("error processing CU output: %w", processErr)
	}

	if cmdErr != nil {
		// Check if this is due to context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg command failed: %w", cmdErr)
	}

	return nil
}

// CheckCodecCompatibility verifies if a video file has a codec supported for CU analysis.
// It provides a public interface to the private checkCodecCompatibility method,
// allowing external code to verify compatibility before attempting analysis.
// Returns nil if compatible, an error with details otherwise.
func (ca *CUAnalyzer) CheckCodecCompatibility(filePath string) error {
	return ca.checkCodecCompatibility(filePath)
}

// NewCUAnalyzer creates a new CU analyzer instance with the provided FFmpeg configuration.
// It verifies that FFmpeg is properly installed and available, and validates that a prober
// is available for codec detection. Returns an initialized CUAnalyzer ready to extract
// coding unit data from video files.
func NewCUAnalyzer(ffmpegInfo *FFmpegInfo, prober *Prober) (*CUAnalyzer, error) {
	if ffmpegInfo == nil || !ffmpegInfo.Installed {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	if prober == nil {
		return nil, fmt.Errorf("prober is required for CU analysis")
	}

	return &CUAnalyzer{
		FFmpegPath: ffmpegInfo.Path,
		prober:     prober,
	}, nil
}
