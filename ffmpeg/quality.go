// Package ffmpeg provides functionality for detecting and working with FFmpeg,
// including tools for analyzing video quality metrics, quantization parameters,
// and other encoder-specific information.
package ffmpeg

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// NewQualityAnalyzer creates a new QualityAnalyzer with the provided FFmpeg configuration.
func NewQualityAnalyzer(ffmpegInfo *FFmpegInfo, prober ProberInterface) (*QualityAnalyzer, error) {
	if ffmpegInfo == nil || !ffmpegInfo.Installed {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	if prober == nil {
		return nil, fmt.Errorf("prober is required for codec detection")
	}

	// Create a QPAnalyzer if QP analysis is supported
	var qpAnalyzer *QPAnalyzer
	if ffmpegInfo.HasQPReadingInfoSupport {
		var err error
		qpAnalyzer, err = NewQPAnalyzer(ffmpegInfo, prober)
		if err != nil {
			return nil, fmt.Errorf("failed to create QP analyzer: %w", err)
		}
	}

	// List of supported codecs for quality analysis
	supportedCodecs := []string{
		qualityCodecDivx,
		qualityCodecXvid,
		qualityCodecH264,
		qualityCodecHEVC,
		qualityCodecMPEG4,
	}

	return &QualityAnalyzer{
		FFmpegPath:      ffmpegInfo.Path,
		SupportedCodecs: supportedCodecs,
		prober:          prober,
		qpAnalyzer:      qpAnalyzer,
	}, nil
}

// IsCodecSupported checks if the given video file has a codec that supports quality analysis.
// Returns nil if the codec is supported, otherwise returns an error with details.
func (qa *QualityAnalyzer) IsCodecSupported(filePath string) error {
	// Get container info to check codec type
	containerInfo, err := qa.prober.GetExtendedContainerInfo(filePath)
	if err != nil {
		return fmt.Errorf("failed to get container info: %w", err)
	}

	// Check if we have video streams
	if len(containerInfo.VideoStreams) == 0 {
		return fmt.Errorf("no video streams found in file")
	}

	// Check if any video stream has a compatible codec
	for _, stream := range containerInfo.VideoStreams {
		codec := strings.ToLower(stream.Format)

		// Check against supported codecs
		for _, supportedCodec := range qa.SupportedCodecs {
			if codec == supportedCodec ||
				(supportedCodec == qualityCodecXvid && strings.Contains(codec, "xvid")) ||
				(supportedCodec == qualityCodecDivx && strings.Contains(codec, "divx")) {
				return nil
			}
		}
	}

	// No compatible codec found
	return fmt.Errorf("no compatible video codec found for quality analysis. Supported codecs: %s",
		strings.Join(qa.SupportedCodecs, ", "))
}

// GenerateQualityReport creates a comprehensive quality analysis report for a video file.
func (qa *QualityAnalyzer) GenerateQualityReport(ctx context.Context, filePath string) (*QualityReport, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", err)
	}

	// Check if codec is supported
	if err := qa.IsCodecSupported(filePath); err != nil {
		return nil, err
	}

	// Get video information
	videoInfo, err := qa.getVideoInfo(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	// Create a quality report
	report := &QualityReport{
		Filename:  filepath.Base(filePath),
		VideoInfo: *videoInfo,
	}

	// Generate QP report if supported
	if qa.qpAnalyzer != nil {
		qpReport, err := qa.qpAnalyzer.GenerateQPReport(ctx, filePath)
		if err == nil {
			// Create a summary of the QP report
			report.QPReportSummary = &QPReportSummary{
				TotalFrames:     qpReport.TotalFrames,
				AverageQP:       qpReport.AverageQP,
				MinQP:           qpReport.MinQP,
				MaxQP:           qpReport.MaxQP,
				CodecType:       qpReport.CodecType,
				Percentiles:     qpReport.Percentiles,
				AverageQPByType: qpReport.AverageQPByType,
			}
		}
	}

	return report, nil
}

// GenerateQualityReportJSON creates a quality analysis report and returns it as a JSON string.
func (qa *QualityAnalyzer) GenerateQualityReportJSON(ctx context.Context, filePath string) (string, error) {
	report, err := qa.GenerateQualityReport(ctx, filePath)
	if err != nil {
		return "", err
	}

	// Convert the report to JSON
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to convert report to JSON: %w", err)
	}

	return string(jsonData), nil
}

// CalculatePSNR calculates PSNR between the input video and a reference video.
// If referenceFile is empty, this function returns nil metrics.
func (qa *QualityAnalyzer) CalculatePSNR(ctx context.Context, inputFile, referenceFile string) (*PSNRMetrics, error) {
	if referenceFile == "" {
		return nil, nil
	}

	// Check if files exist
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file not found: %s", err)
	}
	if _, err := os.Stat(referenceFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("reference file not found: %s", err)
	}

	// Build the FFmpeg command for PSNR calculation
	cmd := exec.CommandContext(
		ctx,
		qa.FFmpegPath,
		"-i", inputFile,
		"-i", referenceFile,
		"-lavfi", "psnr=stats_file=-",
		"-f", "null",
		"-",
	)

	// Create a pipe for stderr output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Create a pipe for stdout output (where PSNR stats go)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Read PSNR data from stdout
	psnrData, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read PSNR data: %w", err)
	}

	// Wait for the command to complete
	if err := cmd.Wait(); err != nil {
		// Read stderr for more info on the error
		stderrData, _ := io.ReadAll(stderr)
		return nil, fmt.Errorf("ffmpeg command failed: %w\nDetails: %s", err, stderrData)
	}

	// Parse PSNR data
	return qa.parsePSNROutput(string(psnrData))
}

// CalculateSSIM calculates SSIM between the input video and a reference video.
// If referenceFile is empty, this function returns nil metrics.
func (qa *QualityAnalyzer) CalculateSSIM(ctx context.Context, inputFile, referenceFile string) (*SSIMMetrics, error) {
	if referenceFile == "" {
		return nil, nil
	}

	// Check if files exist
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file not found: %s", err)
	}
	if _, err := os.Stat(referenceFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("reference file not found: %s", err)
	}

	// Build the FFmpeg command for SSIM calculation
	cmd := exec.CommandContext(
		ctx,
		qa.FFmpegPath,
		"-i", inputFile,
		"-i", referenceFile,
		"-lavfi", "ssim=stats_file=-",
		"-f", "null",
		"-",
	)

	// Create a pipe for stderr output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Create a pipe for stdout output (where SSIM stats go)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Read SSIM data from stdout
	ssimData, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSIM data: %w", err)
	}

	// Wait for the command to complete
	if err := cmd.Wait(); err != nil {
		// Read stderr for more info on the error
		stderrData, _ := io.ReadAll(stderr)
		return nil, fmt.Errorf("ffmpeg command failed: %w\nDetails: %s", err, stderrData)
	}

	// Parse SSIM data
	return qa.parseSSIMOutput(string(ssimData))
}

// CalculateVMAF calculates VMAF between the input video and a reference video.
// If referenceFile is empty, this function returns nil metrics.
func (qa *QualityAnalyzer) CalculateVMAF(ctx context.Context, inputFile, referenceFile string) (*VMAFMetrics, error) {
	if referenceFile == "" {
		return nil, nil
	}

	// Check if files exist
	if _, err := os.Stat(inputFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("input file not found: %s", err)
	}
	if _, err := os.Stat(referenceFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("reference file not found: %s", err)
	}

	// Build the FFmpeg command for VMAF calculation
	cmd := exec.CommandContext(
		ctx,
		qa.FFmpegPath,
		"-i", inputFile,
		"-i", referenceFile,
		"-lavfi", "libvmaf=log_fmt=json:log_path=-",
		"-f", "null",
		"-",
	)

	// Create a pipe for stderr output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Create a pipe for stdout output (where VMAF stats go)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Read VMAF data from stdout
	vmafData, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read VMAF data: %w", err)
	}

	// Wait for the command to complete
	if err := cmd.Wait(); err != nil {
		// Read stderr for more info on the error
		stderrData, _ := io.ReadAll(stderr)
		return nil, fmt.Errorf("ffmpeg command failed: %w\nDetails: %s", err, stderrData)
	}

	// Parse VMAF data
	return qa.parseVMAFOutput(string(vmafData))
}

// CalculateAllMetrics calculates all supported quality metrics if a reference file is provided.
func (qa *QualityAnalyzer) CalculateAllMetrics(ctx context.Context, inputFile, referenceFile string) (*QualityMetrics, error) {
	if referenceFile == "" {
		return &QualityMetrics{}, nil
	}

	metrics := &QualityMetrics{}

	// Calculate PSNR
	psnr, err := qa.CalculatePSNR(ctx, inputFile, referenceFile)
	if err == nil {
		metrics.PSNR = psnr
	}

	// Calculate SSIM
	ssim, err := qa.CalculateSSIM(ctx, inputFile, referenceFile)
	if err == nil {
		metrics.SSIM = ssim
	}

	// Calculate VMAF
	vmaf, err := qa.CalculateVMAF(ctx, inputFile, referenceFile)
	if err == nil {
		metrics.VMAF = vmaf
	}

	return metrics, nil
}

// GenerateCompleteQualityReport creates a comprehensive quality report including
// QP analysis and all quality metrics if a reference file is provided.
func (qa *QualityAnalyzer) GenerateCompleteQualityReport(ctx context.Context, inputFile, referenceFile string) (*QualityReport, error) {
	// Generate base quality report
	report, err := qa.GenerateQualityReport(ctx, inputFile)
	if err != nil {
		return nil, err
	}

	// Calculate quality metrics if reference file is provided
	if referenceFile != "" {
		metrics, err := qa.CalculateAllMetrics(ctx, inputFile, referenceFile)
		if err == nil {
			report.QualityMetrics = *metrics
		}
	}

	return report, nil
}

// Private methods

// getVideoInfo extracts basic video information from a file.
func (qa *QualityAnalyzer) getVideoInfo(filePath string) (*VideoInfo, error) {
	// Get container info
	containerInfo, err := qa.prober.GetExtendedContainerInfo(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get container info: %w", err)
	}

	// Check if we have video streams
	if len(containerInfo.VideoStreams) == 0 {
		return nil, fmt.Errorf("no video streams found in file")
	}

	// Get the first video stream
	videoStream := containerInfo.VideoStreams[0]

	return &VideoInfo{
		Codec:     videoStream.Format,
		Width:     videoStream.Width,
		Height:    videoStream.Height,
		FrameRate: videoStream.FrameRate,
		BitRate:   videoStream.BitRate,
		Duration:  videoStream.Duration,
	}, nil
}

// parsePSNROutput parses the output from FFmpeg's PSNR filter.
func (qa *QualityAnalyzer) parsePSNROutput(output string) (*PSNRMetrics, error) {
	// Create metrics object
	metrics := &PSNRMetrics{}

	// Define regex patterns for parsing
	avgPattern := regexp.MustCompile(`average:(\d+\.\d+)`)
	yPattern := regexp.MustCompile(`Y:(\d+\.\d+)`)
	uPattern := regexp.MustCompile(`U:(\d+\.\d+)`)
	vPattern := regexp.MustCompile(`V:(\d+\.\d+)`)

	// Extract metrics
	if match := avgPattern.FindStringSubmatch(output); len(match) > 1 {
		val, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			metrics.Average = val
		}
	}

	if match := yPattern.FindStringSubmatch(output); len(match) > 1 {
		val, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			metrics.Y = val
		}
	}

	if match := uPattern.FindStringSubmatch(output); len(match) > 1 {
		val, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			metrics.U = val
		}
	}

	if match := vPattern.FindStringSubmatch(output); len(match) > 1 {
		val, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			metrics.V = val
		}
	}

	return metrics, nil
}

// parseSSIMOutput parses the output from FFmpeg's SSIM filter.
func (qa *QualityAnalyzer) parseSSIMOutput(output string) (*SSIMMetrics, error) {
	// Create metrics object
	metrics := &SSIMMetrics{}

	// Define regex patterns for parsing
	avgPattern := regexp.MustCompile(`All:(\d+\.\d+)`)
	yPattern := regexp.MustCompile(`Y:(\d+\.\d+)`)
	uPattern := regexp.MustCompile(`U:(\d+\.\d+)`)
	vPattern := regexp.MustCompile(`V:(\d+\.\d+)`)

	// Extract metrics
	if match := avgPattern.FindStringSubmatch(output); len(match) > 1 {
		val, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			metrics.Average = val
		}
	}

	if match := yPattern.FindStringSubmatch(output); len(match) > 1 {
		val, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			metrics.Y = val
		}
	}

	if match := uPattern.FindStringSubmatch(output); len(match) > 1 {
		val, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			metrics.U = val
		}
	}

	if match := vPattern.FindStringSubmatch(output); len(match) > 1 {
		val, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			metrics.V = val
		}
	}

	return metrics, nil
}

// parseVMAFOutput parses the JSON output from FFmpeg's libvmaf filter.
func (qa *QualityAnalyzer) parseVMAFOutput(output string) (*VMAFMetrics, error) {
	// Create metrics object
	metrics := &VMAFMetrics{}

	// Parse VMAF JSON output
	type vmafResult struct {
		Pooled struct {
			Mean float64 `json:"mean"`
			Min  float64 `json:"min"`
			Max  float64 `json:"max"`
		} `json:"pooled_metrics"`
	}

	var result vmafResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		return nil, fmt.Errorf("failed to parse VMAF output: %w", err)
	}

	// Set metrics
	metrics.Score = result.Pooled.Mean
	metrics.Min = result.Pooled.Min
	metrics.Max = result.Pooled.Max

	return metrics, nil
}

// QPAnalyzer public methods (alphabetical)

// AnalyzeQP extracts frame-by-frame QP (Quantization Parameter) data from a video file.
// It runs FFmpeg with debug flags to capture QP values for each frame,
// processes the output to extract frame type, number, and QP values,
// and streams the results through the provided channel. This allows for detailed
// analysis of encoding quality throughout the video file.
func (qa *QPAnalyzer) AnalyzeQP(ctx context.Context, filePath string, resultCh chan<- FrameQP) error {
	// Verify FFmpeg is available
	if qa.FFmpegPath == "" {
		return fmt.Errorf("ffmpeg not available for QP analysis")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", err)
	}

	// Create a child context with cancellation to manage the command lifecycle
	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Build the FFmpeg command with the appropriate debug options for QP extraction
	cmd := exec.CommandContext(
		childCtx,
		qa.FFmpegPath,
		"-hide_banner",
		"-loglevel", "debug", // Use debug level to see QP values
		"-i", filePath,
		"-f", "null",
		"-",
	)

	// Create a pipe for stderr output where FFmpeg writes debug info
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Set up a channel to pass errors from the processing goroutine
	errCh := make(chan error, 1)

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// Process the output in a separate goroutine
	go func() {
		err := qa.processQPOutput(childCtx, stderr, resultCh)
		errCh <- err
	}()

	// Wait for either completion or context cancellation
	var processErr error
	select {
	case processErr = <-errCh:
		// Processing completed
	case <-ctx.Done():
		// Context canceled
		cancel()
		<-errCh // Wait for processing goroutine to complete
		return ctx.Err()
	}

	// Check for processing errors
	if processErr != nil {
		return fmt.Errorf("error processing QP output: %w", processErr)
	}

	// Wait for the command to complete
	if err := cmd.Wait(); err != nil {
		// Check if this is due to context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg command failed: %w", err)
	}

	return nil
}

// GenerateQPReport creates a comprehensive QP analysis report for a video file.
// It analyzes the video file and generates statistics about QP values,
// including average, min, max, and distribution of QP values across different frame types.
func (qa *QPAnalyzer) GenerateQPReport(ctx context.Context, filePath string) (*QPReport, error) {
	// Create result channel for FrameQP objects
	resultCh := make(chan FrameQP, qpChannelBufferSize)

	// Create a report structure
	report := qa.initQPReport(filePath)

	// Create a goroutine to collect frames from the channel
	done := make(chan struct{})
	go func() {
		defer close(done)

		for frame := range resultCh {
			qa.processFrameForReport(report, frame)
		}

		// Calculate final statistics
		qa.calculateReportStatistics(report)
	}()

	// Start the QP analysis in a separate goroutine
	err := qa.AnalyzeQP(ctx, filePath, resultCh)

	// Wait for all frames to be processed
	<-done

	if err != nil {
		return nil, fmt.Errorf("error analyzing QP: %w", err)
	}

	// Verify that we have at least one frame
	if report.TotalFrames == 0 {
		return nil, fmt.Errorf("no frames were analyzed, possibly unsupported codec or corrupted file")
	}

	return report, nil
}

// GenerateQPReportJSON creates a QP analysis report and returns it as a JSON string.
// This is a convenience method for applications that need to serialize the report.
func (qa *QPAnalyzer) GenerateQPReportJSON(ctx context.Context, filePath string) (string, error) {
	report, err := qa.GenerateQPReport(ctx, filePath)
	if err != nil {
		return "", err
	}

	// Convert the report to JSON
	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to convert report to JSON: %w", err)
	}

	return string(jsonData), nil
}

// IsCodecSupported checks if the given video file has a codec that supports QP analysis.
// Returns nil if the codec is supported, otherwise returns an error with details.
func (qa *QPAnalyzer) IsCodecSupported(filePath string) error {
	// Get container info to check codec type
	containerInfo, err := qa.prober.GetExtendedContainerInfo(filePath)
	if err != nil {
		return fmt.Errorf("failed to get container info: %w", err)
	}

	// Check if we have video streams
	if len(containerInfo.VideoStreams) == 0 {
		return fmt.Errorf("no video streams found in file")
	}

	// Check if any video stream has a compatible codec
	for _, stream := range containerInfo.VideoStreams {
		codec := strings.ToLower(stream.Format)
		if codec == "h264" || codec == "avc" || codec == "hevc" || codec == "h265" {
			return nil
		}
	}

	// No compatible codec found
	return fmt.Errorf("no compatible video codec found for QP analysis. Only H.264/AVC and H.265/HEVC are supported")
}

// NewQPAnalyzer creates a new QP analyzer instance with the provided FFmpeg configuration.
// It verifies that FFmpeg is properly installed and available, and initializes the
// regular expressions used for parsing QP debug output.
func NewQPAnalyzer(ffmpegInfo *FFmpegInfo, prober ProberInterface) (*QPAnalyzer, error) {
	if ffmpegInfo == nil || !ffmpegInfo.Installed {
		return nil, fmt.Errorf("ffmpeg not available")
	}

	if prober == nil {
		return nil, fmt.Errorf("prober is required for codec detection")
	}

	return &QPAnalyzer{
		FFmpegPath:         ffmpegInfo.Path,
		SupportsQPAnalysis: ffmpegInfo.HasQPReadingInfoSupport,
		prober:             prober,

		// Generic patterns (fallback)
		genericNewFrameRegex:  regexp.MustCompile(`(?i)New\s+(frame|picture)`),
		genericFrameTypeRegex: regexp.MustCompile(`(?i)(i|p|b)(\s|-|_)frame`),
		genericQPRegex:        regexp.MustCompile(`\b(QP=\d+|\d{1,2})\b`),

		// H.264 specific patterns
		h264NewFrameRegex:  regexp.MustCompile(`(?i)New\s+picture|pict_type\s*:\s*[IPB]`),
		h264FrameTypeRegex: regexp.MustCompile(`(?i)pict_type\s*:\s*([IPB])`),
		h264QPRegex:        regexp.MustCompile(`(?i)(QP\s*=\s*\d+|\b\d{1,2}\b)`),

		// HEVC specific patterns
		hevcNewFrameRegex:  regexp.MustCompile(`(?i)New\s+frame|nal_unit_type\s*:\s*\d+\s*\([^)]+\)`),
		hevcFrameTypeRegex: regexp.MustCompile(`(?i)SLICE_(I|P|B)`),
		hevcQPRegex:        regexp.MustCompile(`(?i)(QP\s*=\s*\d+|\bQP\d+\b|\b\d{1,2}\b)`),
	}, nil
}

// processQPOutput processes the output from FFmpeg QP debug mode.
// It handles both H.264 and HEVC formats, extracting QP values on a frame-by-frame basis
// and streaming the results through the provided channel.
// This method is part of the QPAnalyzer and is called from the AnalyzeQP method.
func (qa *QPAnalyzer) processQPOutput(ctx context.Context, reader io.Reader, resultCh chan<- FrameQP) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), scannerBufferSize)

	var currentFrame *FrameQP
	var frameNumber int = 0
	var qpValues []int
	var codecType string = ""

	// Process each line from FFmpeg output
	for scanner.Scan() {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}

		line := scanner.Text()

		// Detect codec type if not already determined
		if codecType == "" {
			codecType = qa.detectCodecType(line)
		}

		// Check if this is a new frame
		if qa.isNewFrameLine(line, codecType) {
			// If we have a current frame, finalize and send it
			if err := qa.finalizeAndSendFrame(ctx, currentFrame, qpValues, codecType, resultCh); err != nil {
				return err
			}

			// Start a new frame
			frameNumber++
			currentFrame, qpValues = qa.createNewFrame(frameNumber, line, codecType)
		} else {
			// Parse QP values if this is not a new frame line
			newValues := qa.parseQPValues(line, codecType)
			qpValues = append(qpValues, newValues...)
		}
	}

	// Handle any remaining frame data at the end
	if err := qa.finalizeAndSendFrame(ctx, currentFrame, qpValues, codecType, resultCh); err != nil {
		return err
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading FFmpeg output: %w", err)
	}

	return nil
}

// finalizeAndSendFrame calculates the average QP for a frame and sends it to the result channel
func (qa *QPAnalyzer) finalizeAndSendFrame(ctx context.Context, frame *FrameQP, qpValues []int, codecType string, resultCh chan<- FrameQP) error {
	if frame == nil || len(qpValues) == 0 {
		return nil
	}

	// Calculate average QP and send the frame
	frame.QPValues = qpValues
	frame.AverageQP = qa.calculateAverageQP(qpValues)
	frame.CodecType = codecType

	select {
	case resultCh <- *frame:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// createNewFrame creates a new FrameQP object from a frame line
func (qa *QPAnalyzer) createNewFrame(frameNumber int, line string, codecType string) (*FrameQP, []int) {
	frameType := qa.extractFrameType(line, codecType)
	currentFrame := &FrameQP{
		FrameNumber: frameNumber,
		FrameType:   frameType,
	}
	return currentFrame, []int{}
}

// QPAnalyzer private methods (alphabetical)

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

// detectCodecType analyzes the line to detect the codec type (h264 or hevc).
// Returns the detected codec type or an empty string if none is detected.
func (qa *QPAnalyzer) detectCodecType(line string) string {
	lowerLine := strings.ToLower(line)

	if strings.Contains(lowerLine, "h264") || strings.Contains(lowerLine, "avc") {
		return codecH264
	} else if strings.Contains(lowerLine, "h265") || strings.Contains(lowerLine, "hevc") {
		return codecHEVC
	}

	return ""
}

// extractFrameType extracts the frame type (I, P, or B) from a frame header line.
// It matches different patterns depending on the codec type.
// Returns "?" if the frame type cannot be determined.
func (qa *QPAnalyzer) extractFrameType(line string, codecType string) string {
	var matches []string

	switch codecType {
	case codecH264:
		matches = qa.h264FrameTypeRegex.FindStringSubmatch(line)
	case codecHEVC:
		matches = qa.hevcFrameTypeRegex.FindStringSubmatch(line)
	default:
		matches = qa.genericFrameTypeRegex.FindStringSubmatch(line)
	}

	if len(matches) > 1 {
		frameType := strings.ToUpper(matches[1])
		if frameType == "I" || frameType == "P" || frameType == "B" {
			return frameType
		}
	}

	return "?"
}

// extractQPFromLine extracts the QP values from an output line based on the codec type.
// It uses different regex patterns for H.264 and HEVC.
// Returns a slice of QP values and a boolean indicating if any values were found.
func (qa *QPAnalyzer) extractQPFromLine(line string, codecType string) ([]int, bool) {
	var qpMatches []string

	switch codecType {
	case codecH264:
		qpMatches = qa.h264QPRegex.FindAllString(line, -1)
	case codecHEVC:
		qpMatches = qa.hevcQPRegex.FindAllString(line, -1)
	default:
		qpMatches = qa.genericQPRegex.FindAllString(line, -1)
	}

	if len(qpMatches) == 0 {
		return nil, false
	}

	// Convert matched strings to integers
	qpValues := make([]int, 0, len(qpMatches))
	for _, match := range qpMatches {
		// Try to extract QP value from patterns like "QP=23"
		if strings.Contains(match, "=") {
			parts := strings.Split(match, "=")
			if len(parts) >= 2 {
				match = parts[1]
			}
		}

		// Extract pure numbers
		numStr := regexp.MustCompile(`\d+`).FindString(match)
		if numStr == "" {
			continue
		}

		qp, err := strconv.Atoi(numStr)
		if err == nil && qp >= 0 && qp <= 51 { // QP values are typically between 0 and 51
			qpValues = append(qpValues, qp)
		}
	}

	return qpValues, len(qpValues) > 0
}

// isNewFrameLine determines if a line indicates a new frame in the FFmpeg debug output.
// It checks codec-specific patterns for H.264 and HEVC.
func (qa *QPAnalyzer) isNewFrameLine(line string, codecType string) bool {
	switch codecType {
	case codecH264:
		return qa.h264NewFrameRegex.MatchString(line)
	case codecHEVC:
		return qa.hevcNewFrameRegex.MatchString(line)
	default:
		return qa.genericNewFrameRegex.MatchString(line)
	}
}

// initQPReport creates and initializes a new QPReport structure
func (qa *QPAnalyzer) initQPReport(filePath string) *QPReport {
	return &QPReport{
		Filename: filepath.Base(filePath),
		FrameData: map[string][]FrameQP{
			"I": {},
			"P": {},
			"B": {},
		},
		AverageQPByType: make(map[string]float64),
		CodecType:       "",
	}
}

// processFrameForReport updates the report with data from a frame
func (qa *QPAnalyzer) processFrameForReport(report *QPReport, frame FrameQP) {
	// Set the codec type in the report
	if report.CodecType == "" && frame.CodecType != "" {
		report.CodecType = frame.CodecType
	}

	// Store frame by type
	if frame.FrameType == "I" || frame.FrameType == "P" || frame.FrameType == "B" {
		report.FrameData[frame.FrameType] = append(report.FrameData[frame.FrameType], frame)
	}

	// Track overall stats
	report.TotalFrames++
	report.TotalQP += frame.AverageQP
	report.QPValues = append(report.QPValues, frame.QPValues...)

	// Update min/max QP values
	if frame.AverageQP < report.MinQP || report.MinQP == 0 {
		report.MinQP = frame.AverageQP
	}
	if frame.AverageQP > report.MaxQP {
		report.MaxQP = frame.AverageQP
	}
}

// calculateReportStatistics computes final statistics for the report
func (qa *QPAnalyzer) calculateReportStatistics(report *QPReport) {
	// Calculate overall average
	if report.TotalFrames > 0 {
		report.AverageQP = report.TotalQP / float64(report.TotalFrames)
	}

	// Calculate averages by frame type
	for frameType, frames := range report.FrameData {
		if len(frames) > 0 {
			var sum float64
			for _, frame := range frames {
				sum += frame.AverageQP
			}
			report.AverageQPByType[frameType] = sum / float64(len(frames))
		}
	}
}

// parseQPValues extracts QP values from a line of FFmpeg output
func (qa *QPAnalyzer) parseQPValues(line string, codecType string) []int {
	values, _ := qa.extractQPFromLine(line, codecType)
	return values
}
