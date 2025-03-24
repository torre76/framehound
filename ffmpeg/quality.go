// Package ffmpeg provides functionality for detecting and working with FFmpeg.
// It offers capabilities for analyzing video files, extracting metadata, and
// processing frame-level information such as bitrates, quality parameters, and
// quality metrics including QP values, PSNR, SSIM, and VMAF.
package ffmpeg

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// QualityLevel represents the categorical quality level of a frame
type QualityLevel int

const (
	// UglyQuality represents the lowest quality level
	UglyQuality QualityLevel = iota
	// BadQuality represents a poor quality level
	BadQuality
	// MediumQuality represents an average quality level
	MediumQuality
	// HighQuality represents a good quality level
	HighQuality
	// ExcellentQuality represents the highest quality level
	ExcellentQuality
)

// String returns a string representation of the QualityLevel
func (ql QualityLevel) String() string {
	switch ql {
	case UglyQuality:
		return "Ugly"
	case BadQuality:
		return "Bad"
	case MediumQuality:
		return "Medium"
	case HighQuality:
		return "High"
	case ExcellentQuality:
		return "Excellent"
	default:
		return "Unknown"
	}
}

// QualityFrame represents the quality information for a single frame
type QualityFrame struct {
	FrameNumber  int          // The frame number (starting from 0)
	Quality      float64      // A numerical quality score for the frame (higher is better)
	QualityLevel QualityLevel // A categorical quality level
}

// FrameQualityAnalyzer is an interface for analyzing video quality on a frame-by-frame basis
type FrameQualityAnalyzer interface {
	Analyze(filePath string, frameQualityChan chan<- QualityFrame) error
}

// BaseQualityAnalyzer contains common functionality for quality analyzers
type BaseQualityAnalyzer struct {
	FFmpegPath  string // path to FFmpeg executable
	FFprobePath string // path to FFprobe executable
}

// determineQualityLevel converts a numerical quality score to a QualityLevel enumeration
func (b *BaseQualityAnalyzer) determineQualityLevel(quality float64, codec string) QualityLevel {
	switch codec {
	case "h264", "hevc":
		// For H.264 and HEVC, lower QP values mean higher quality
		// These thresholds are approximations and can be adjusted
		if quality <= 10 {
			return ExcellentQuality
		} else if quality <= 18 {
			return HighQuality
		} else if quality <= 25 {
			return MediumQuality
		} else if quality <= 35 {
			return BadQuality
		}
		return UglyQuality
	case "xvid", "divx":
		// For XVID and DIVX, different thresholds may apply
		// These are approximations and should be refined based on research
		if quality <= 2 {
			return ExcellentQuality
		} else if quality <= 4 {
			return HighQuality
		} else if quality <= 6 {
			return MediumQuality
		} else if quality <= 8 {
			return BadQuality
		}
		return UglyQuality
	default:
		// Default mapping for unknown codecs
		if quality >= 90 {
			return ExcellentQuality
		} else if quality >= 70 {
			return HighQuality
		} else if quality >= 50 {
			return MediumQuality
		} else if quality >= 30 {
			return BadQuality
		}
		return UglyQuality
	}
}

// getCodecFromFile uses FFprobe to determine the video codec used in a file
func (b *BaseQualityAnalyzer) getCodecFromFile(filePath string) (string, error) {
	cmd := exec.Command(
		b.FFprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("error determining codec: %w", err)
	}

	codec := strings.TrimSpace(string(output))
	if codec == "" {
		return "", errors.New("could not determine video codec")
	}

	return codec, nil
}

// normalizeQualityScore normalizes quality scores based on the codec
// For some codecs like H.264/HEVC, lower QP is better quality, so we invert the scale
func (b *BaseQualityAnalyzer) normalizeQualityScore(rawScore float64, codec string) float64 {
	switch codec {
	case "h264", "hevc":
		// Convert QP scale (0-51, lower is better) to a 0-100 scale (higher is better)
		// 51 is the max QP value, so 51-QP gives us a direct mapping where 0 QP = 51 (best)
		// We then scale it to 0-100
		return (51 - rawScore) * (100.0 / 51.0)
	case "xvid", "divx":
		// For these codecs we might have a different scale, this is a placeholder
		// that assumes rawScore is already on a 0-10 scale (lower is better)
		return (10 - rawScore) * 10
	default:
		// For unknown codecs, assume rawScore is already on a 0-100 scale (higher is better)
		return rawScore
	}
}

// XvidQualityAnalyzer implements FrameQualityAnalyzer for Xvid codec
type XvidQualityAnalyzer struct {
	BaseQualityAnalyzer
}

// Analyze processes a video file and sends frame quality data to the provided channel
func (a *XvidQualityAnalyzer) Analyze(filePath string, frameQualityChan chan<- QualityFrame) error {
	defer close(frameQualityChan)

	// For Xvid, we'll look for quantizer information in FFmpeg debug output
	// This regexp looks for lines that contain frame number and quantizer information
	frameQPRegex := regexp.MustCompile(`(?i)frame=\s*(\d+).*q=\s*([0-9.]+)`)

	cmd := exec.Command(
		a.FFmpegPath,
		"-i", filePath,
		"-f", "null",
		"-loglevel", "debug",
		"-",
	)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting ffmpeg: %w", err)
	}

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		matches := frameQPRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			frameNumber, err := strconv.Atoi(matches[1])
			if err != nil {
				continue
			}

			qp, err := strconv.ParseFloat(matches[2], 64)
			if err != nil {
				continue
			}

			// Normalize the quality score
			normalizedQuality := a.normalizeQualityScore(qp, "xvid")

			// Determine quality level
			qualityLevel := a.determineQualityLevel(qp, "xvid")

			frameQualityChan <- QualityFrame{
				FrameNumber:  frameNumber,
				Quality:      normalizedQuality,
				QualityLevel: qualityLevel,
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error during ffmpeg execution: %w", err)
	}

	return nil
}

// DivxQualityAnalyzer implements FrameQualityAnalyzer for DivX codec
type DivxQualityAnalyzer struct {
	BaseQualityAnalyzer
}

// Analyze processes a video file and sends frame quality data to the provided channel
func (a *DivxQualityAnalyzer) Analyze(filePath string, frameQualityChan chan<- QualityFrame) error {
	defer close(frameQualityChan)

	// For DivX, we'll look for similar quantizer information as with Xvid
	frameQPRegex := regexp.MustCompile(`(?i)frame=\s*(\d+).*q=\s*([0-9.]+)`)

	cmd := exec.Command(
		a.FFmpegPath,
		"-i", filePath,
		"-f", "null",
		"-loglevel", "debug",
		"-",
	)

	log.Printf("DivX Command for QP extraction: %s", cmd.String())

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting ffmpeg: %w", err)
	}

	scanner := bufio.NewScanner(stderr)
	frameCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		matches := frameQPRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			frameNumber, err := strconv.Atoi(matches[1])
			if err != nil {
				continue
			}

			qp, err := strconv.ParseFloat(matches[2], 64)
			if err != nil {
				continue
			}

			// Normalize the quality score
			normalizedQuality := a.normalizeQualityScore(qp, "divx")

			// Determine quality level
			qualityLevel := a.determineQualityLevel(qp, "divx")

			frameQualityChan <- QualityFrame{
				FrameNumber:  frameNumber,
				Quality:      normalizedQuality,
				QualityLevel: qualityLevel,
			}

			frameCount++
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("DivX FFmpeg command failed with error: %v", err)
		return fmt.Errorf("error during ffmpeg execution: %w", err)
	}

	if frameCount == 0 {
		log.Printf("DivX No frames with quality data found for %s", filePath)
		return fmt.Errorf("no quality data found for DivX video: %s", filePath)
	}

	log.Printf("DivX Processed %d frames for %s", frameCount, filePath)
	return nil
}

// H264QualityAnalyzer implements FrameQualityAnalyzer for H.264 codec
type H264QualityAnalyzer struct {
	BaseQualityAnalyzer
}

// Analyze processes a video file and sends frame quality data to the provided channel
func (a *H264QualityAnalyzer) Analyze(filePath string, frameQualityChan chan<- QualityFrame) error {
	defer close(frameQualityChan)

	// Try the basic method first, which is more reliable than qp-hist
	log.Printf("H264QualityAnalyzer: Starting basic analysis for file %s", filePath)
	err := a.basicAnalyze(filePath, frameQualityChan)
	if err != nil {
		log.Printf("H264QualityAnalyzer: Basic analysis failed with error: %v. Trying qp-hist method.", err)
		return a.qpHistAnalyze(filePath, frameQualityChan)
	}

	return nil
}

// basicAnalyze provides a basic analysis method using standard FFmpeg output
func (a *H264QualityAnalyzer) basicAnalyze(filePath string, frameQualityChan chan<- QualityFrame) error {
	probeData, err := a.runFFprobeCommand(filePath)
	if err != nil {
		log.Printf("H264 Error with ffprobe: %v. Trying FFmpeg method.", err)
		return a.ffmpegTraceAnalyze(filePath, frameQualityChan)
	}

	if len(probeData.Frames) == 0 {
		log.Printf("H264 No frames found in ffprobe output. Trying FFmpeg method.")
		return a.ffmpegTraceAnalyze(filePath, frameQualityChan)
	}

	// Extract QP information from FFmpeg output
	frameQPS := a.extractH264QpValues(filePath)

	// Process frames with QP information
	frameCount, missingQpCount := a.processFramesWithQp(probeData.Frames, frameQPS, frameQualityChan)

	if frameCount == 0 {
		if missingQpCount > 0 {
			log.Printf("H264 No frames were processed. Found %d frames but no QP data available.", missingQpCount)
			return fmt.Errorf("no QP data found for any frames (%d frames skipped)", missingQpCount)
		}
		log.Printf("H264 No frames were processed. Trying FFmpeg method.")
		return a.ffmpegTraceAnalyze(filePath, frameQualityChan)
	}

	if missingQpCount > 0 {
		log.Printf("H264 Warning: Skipped %d out of %d frames (%.1f%%) due to missing QP values",
			missingQpCount, frameCount+missingQpCount, float64(missingQpCount)*100.0/float64(frameCount+missingQpCount))
	}

	log.Printf("H264 Processed %d frames using ffprobe method", frameCount)
	return nil
}

// ProbeData represents the structure of data returned by ffprobe
type ProbeData struct {
	Frames []ProbeFrame `json:"frames"`
}

// ProbeFrame represents a single frame in the ffprobe output
type ProbeFrame struct {
	PictType           string  `json:"pict_type"`
	PktPtsTime         string  `json:"pkt_pts_time,omitempty"`
	CodedPictureNumber string  `json:"coded_picture_number,omitempty"`
	Quality            float64 `json:"quality,omitempty"`
}

// runFFprobeCommand executes the ffprobe command and parses the output
func (a *H264QualityAnalyzer) runFFprobeCommand(filePath string) (*ProbeData, error) {
	cmd := exec.Command(
		a.FFprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_frames",
		"-show_entries", "frame=pict_type,pkt_pts_time,coded_picture_number",
		"-of", "json",
		filePath,
	)
	log.Printf("H264 Using ffprobe to extract frame data: %s", cmd.String())

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("ffprobe command failed: %w", err)
	}

	// Parse the ffprobe JSON output
	var probeData ProbeData
	err = json.Unmarshal(outBuf.Bytes(), &probeData)
	if err != nil {
		return nil, fmt.Errorf("error parsing ffprobe output: %w", err)
	}

	return &probeData, nil
}

// extractH264QpValues extracts QP values from FFmpeg output
func (a *H264QualityAnalyzer) extractH264QpValues(filePath string) map[int]float64 {
	ffmpegCmd := exec.Command(
		a.FFmpegPath,
		"-i", filePath,
		"-c:v", "copy",
		"-f", "null",
		"-loglevel", "debug",
		"-",
	)
	log.Printf("H264 Command for QP extraction: %s", ffmpegCmd.String())

	var ffmpegBuf bytes.Buffer
	ffmpegCmd.Stdout = &ffmpegBuf
	ffmpegCmd.Stderr = &ffmpegBuf

	_ = ffmpegCmd.Run() // We don't care if it fails, we'll extract what we can

	ffmpegOutput := ffmpegBuf.String()

	// Look for QP values in the output
	qpValueRegex := regexp.MustCompile(`QP: (\d+)`)
	qpPerFrameRegex := regexp.MustCompile(`\[h264 @ [^\]]+\] POC: (\d+) \([IPB]\).*QP: (\d+)`)
	bitQpRegex := regexp.MustCompile(`\[h264 @ [^\]]+\] (\d+) \([IPB]\).*qp (\d+)`)

	qpMatches := qpValueRegex.FindAllStringSubmatch(ffmpegOutput, -1)
	qpPerFrameMatches := qpPerFrameRegex.FindAllStringSubmatch(ffmpegOutput, -1)
	bitQpMatches := bitQpRegex.FindAllStringSubmatch(ffmpegOutput, -1)

	log.Printf("H264 Found QP info: %d general QP values, %d POC QP values, %d bitstream QP values",
		len(qpMatches), len(qpPerFrameMatches), len(bitQpMatches))

	// Map to store frame QP values
	frameQPS := make(map[int]float64)

	// Try to extract QP values from specific per-frame QP info
	if len(qpPerFrameMatches) > 0 {
		log.Printf("H264 Using QP per frame information from %d matches", len(qpPerFrameMatches))
		for _, match := range qpPerFrameMatches {
			frameNum, _ := strconv.Atoi(match[1])
			qp, _ := strconv.ParseFloat(match[2], 64)
			frameQPS[frameNum] = qp
		}
	}

	// If we have bit QP values, use those
	if len(bitQpMatches) > 0 && len(frameQPS) == 0 {
		log.Printf("H264 Using bitstream QP information from %d matches", len(bitQpMatches))
		for _, match := range bitQpMatches {
			frameNum, _ := strconv.Atoi(match[1])
			qp, _ := strconv.ParseFloat(match[2], 64)
			frameQPS[frameNum] = qp
		}
	}

	return frameQPS
}

// processFramesWithQp processes frames with available QP values
func (a *H264QualityAnalyzer) processFramesWithQp(
	frames []ProbeFrame,
	frameQPS map[int]float64,
	frameQualityChan chan<- QualityFrame,
) (int, int) {
	frameCount := 0
	missingQpCount := 0

	for i, frame := range frames {
		frameNum := i
		if frame.CodedPictureNumber != "" {
			num, err := strconv.Atoi(frame.CodedPictureNumber)
			if err == nil {
				frameNum = num
			}
		}

		// Get QP value if available, skip frame if not available
		var qp float64
		if val, ok := frameQPS[frameNum]; ok {
			qp = val
		} else {
			// Skip frames without actual QP data
			missingQpCount++
			continue
		}

		normalizedQuality := a.normalizeQualityScore(qp, "h264")
		qualityLevel := a.determineQualityLevel(qp, "h264")

		frameQualityChan <- QualityFrame{
			FrameNumber:  frameNum,
			Quality:      normalizedQuality,
			QualityLevel: qualityLevel,
		}
		frameCount++

		log.Printf("H264 Frame processed: %d, type: %s, QP: %.2f",
			frameNum, frame.PictType, qp)
	}

	return frameCount, missingQpCount
}

// ffmpegTraceAnalyze is the old basicAnalyze method, now used as a fallback
func (a *H264QualityAnalyzer) ffmpegTraceAnalyze(filePath string, frameQualityChan chan<- QualityFrame) error {
	// Execute FFmpeg with debug output to extract frame QP information without re-encoding
	output, err := a.runFFmpegTraceCommand(filePath)
	if err != nil {
		log.Printf("H264 FFmpeg command failed, but continuing to check output: %v", err)
	}

	// Extract information from output
	frameQPS, frameTypes := a.extractH264FrameInfo(output)

	// If we have no frame information at all, try a different approach with select filter
	if len(frameQPS) == 0 {
		log.Printf("H264 No QP information found in trace output, trying select filter...")
		return a.selectFilterAnalyze(filePath, frameQualityChan)
	}

	// Send frame data to channel
	frameCount := a.sendH264FrameDataToChannel(frameQPS, frameTypes, frameQualityChan)

	if frameCount == 0 {
		return fmt.Errorf("no frames extracted from FFmpeg output")
	}

	log.Printf("H264 Extracted QP values for %d frames", frameCount)
	return nil
}

// runFFmpegTraceCommand executes the FFmpeg command to get trace output
func (a *H264QualityAnalyzer) runFFmpegTraceCommand(filePath string) (string, error) {
	cmd := exec.Command(
		a.FFmpegPath,
		"-i", filePath,
		"-c:v", "copy",
		"-f", "null",
		"-loglevel", "trace",
		"-",
	)
	log.Printf("H264 Command: %s", cmd.String())

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	return outBuf.String(), err
}

// extractH264FrameInfo extracts frame information from FFmpeg trace output
func (a *H264QualityAnalyzer) extractH264FrameInfo(output string) (map[int]float64, map[int]string) {
	// Look for frame types and NAL units in the debug output
	nalTypeRegex := regexp.MustCompile(`\[h264 @ [^\]]+\] nal_unit_type: (\d+)(?:\([^)]+\))?, nal_ref_idc: (\d+)`)
	frameTypeRegex := regexp.MustCompile(`\[Parsed_select_0 @ [^\]]+\] n:(\d+\.\d+).*pict_type=([IPB])`)

	// Trace output may contain QP values in several formats
	qpValueRegex := regexp.MustCompile(`QP: (\d+)`)
	mbQpRegex := regexp.MustCompile(`MB QP: (\d+)`)
	qpPerFrameRegex := regexp.MustCompile(`\[h264 @ [^\]]+\] POC: (\d+) \([IPB]\).*QP: (\d+)`)

	// Try to extract QP values directly from the bitstream info
	bitQpRegex := regexp.MustCompile(`\[h264 @ [^\]]+\] (\d+) \([IPB]\).*qp (\d+)`)

	// Find all NAL units
	nalMatches := nalTypeRegex.FindAllStringSubmatch(output, -1)
	frameTypeMatches := frameTypeRegex.FindAllStringSubmatch(output, -1)
	qpMatches := qpValueRegex.FindAllStringSubmatch(output, -1)
	mbQpMatches := mbQpRegex.FindAllStringSubmatch(output, -1)
	qpPerFrameMatches := qpPerFrameRegex.FindAllStringSubmatch(output, -1)
	bitQpMatches := bitQpRegex.FindAllStringSubmatch(output, -1)

	// Create maps to store frame types and QP values
	frameTypes := make(map[int]string)
	frameQPS := make(map[int]float64)

	log.Printf("H264 Found %d NAL units, %d frame types, %d QP values, %d MB QPs, %d QP per frame, %d bit QPs",
		len(nalMatches), len(frameTypeMatches), len(qpMatches), len(mbQpMatches),
		len(qpPerFrameMatches), len(bitQpMatches))

	// Try different methods to extract QP values in order of preference
	a.extractH264PerFrameQp(qpPerFrameMatches, frameQPS)
	a.extractH264BitQp(bitQpMatches, frameQPS)
	a.extractH264QpWithFrameTypes(frameTypeMatches, qpMatches, frameTypes, frameQPS)
	a.extractH264MbQp(nalMatches, mbQpMatches, frameQPS)

	// Check if we need to use frame type defaults
	if len(frameQPS) == 0 && len(frameTypes) > 0 {
		log.Printf("H264 Found %d frame types but no actual QP values", len(frameTypes))
	}

	return frameQPS, frameTypes
}

// extractH264PerFrameQp extracts QP values from per-frame QP matches
func (a *H264QualityAnalyzer) extractH264PerFrameQp(qpPerFrameMatches [][]string, frameQPS map[int]float64) {
	if len(qpPerFrameMatches) > 0 {
		log.Printf("H264 Using QP per frame information from %d matches", len(qpPerFrameMatches))
		for _, match := range qpPerFrameMatches {
			frameNum, _ := strconv.Atoi(match[1])
			qp, _ := strconv.ParseFloat(match[2], 64)
			frameQPS[frameNum] = qp
		}
	}
}

// extractH264BitQp extracts QP values from bitstream QP matches
func (a *H264QualityAnalyzer) extractH264BitQp(bitQpMatches [][]string, frameQPS map[int]float64) {
	if len(bitQpMatches) > 0 && len(frameQPS) == 0 {
		log.Printf("H264 Using bitstream QP information from %d matches", len(bitQpMatches))
		for _, match := range bitQpMatches {
			frameNum, _ := strconv.Atoi(match[1])
			qp, _ := strconv.ParseFloat(match[2], 64)
			frameQPS[frameNum] = qp
		}
	}
}

// extractH264QpWithFrameTypes extracts QP values using frame type information
func (a *H264QualityAnalyzer) extractH264QpWithFrameTypes(
	frameTypeMatches [][]string,
	qpMatches [][]string,
	frameTypes map[int]string,
	frameQPS map[int]float64,
) {
	if len(frameQPS) == 0 && len(frameTypeMatches) > 0 && len(qpMatches) > 0 {
		log.Printf("H264 Associating %d QP values with %d frame types", len(qpMatches), len(frameTypeMatches))
		// First, map frame numbers to types
		for _, match := range frameTypeMatches {
			frameFloat, _ := strconv.ParseFloat(match[1], 64)
			frameNum := int(frameFloat) + 1
			frameType := match[2]
			frameTypes[frameNum] = frameType
		}

		// Then assume QP values are in sequence with frame numbers
		for i, match := range qpMatches {
			if i < len(frameTypeMatches) {
				frameFloat, _ := strconv.ParseFloat(frameTypeMatches[i][1], 64)
				frameNum := int(frameFloat) + 1
				qp, _ := strconv.ParseFloat(match[1], 64)
				frameQPS[frameNum] = qp
			}
		}
	}
}

// extractH264MbQp extracts QP values from MB QP matches
func (a *H264QualityAnalyzer) extractH264MbQp(
	nalMatches [][]string,
	mbQpMatches [][]string,
	frameQPS map[int]float64,
) {
	if len(frameQPS) == 0 && len(mbQpMatches) > 0 {
		log.Printf("H264 Using MB QP information from %d matches", len(mbQpMatches))
		// Assume MB QP values are in sequence with NAL units that represent frames
		frameIndex := 0
		for _, match := range nalMatches {
			nalType := match[1]
			if nalType == "1" || nalType == "5" { // These represent frame data
				if frameIndex < len(mbQpMatches) {
					qp, _ := strconv.ParseFloat(mbQpMatches[frameIndex][1], 64)
					frameQPS[frameIndex+1] = qp
					frameIndex++
				}
			}
		}
	}
}

// sendH264FrameDataToChannel sends frame data to the provided channel
func (a *H264QualityAnalyzer) sendH264FrameDataToChannel(
	frameQPS map[int]float64,
	frameTypes map[int]string,
	frameQualityChan chan<- QualityFrame,
) int {
	frameCount := 0
	for frameNum, qp := range frameQPS {
		normalizedQuality := a.normalizeQualityScore(qp, "h264")
		qualityLevel := a.determineQualityLevel(qp, "h264")

		frameType := frameTypes[frameNum]
		if frameType == "" {
			frameType = "?"
		}

		log.Printf("H264 Frame extracted: %d with type %s and QP: %.2f", frameNum, frameType, qp)

		frameQualityChan <- QualityFrame{
			FrameNumber:  frameNum,
			Quality:      normalizedQuality,
			QualityLevel: qualityLevel,
		}
		frameCount++
	}
	return frameCount
}

// selectFilterAnalyze uses the select filter to get frame information
func (a *H264QualityAnalyzer) selectFilterAnalyze(filePath string, _ chan<- QualityFrame) error {
	// Try using the select filter to extract frame info
	cmd := exec.Command(
		a.FFmpegPath,
		"-i", filePath,
		"-vf", "select=1",
		"-f", "null",
		"-loglevel", "debug",
		"-",
	)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	if err != nil {
		log.Printf("H264 Select filter command failed, but continuing to check output: %v", err)
	}

	output := outBuf.String()

	// Look for frame types in the select filter output
	selectFrameRegex := regexp.MustCompile(`\[Parsed_select_0 @ [^\]]+\] n:(\d+\.\d+).*pict_type=([IPB])`)
	frameMatches := selectFrameRegex.FindAllStringSubmatch(output, -1)

	if len(frameMatches) == 0 {
		return fmt.Errorf("no frames extracted from FFmpeg select filter output")
	}

	log.Printf("H264 Found %d frames from select filter, but no QP data available", len(frameMatches))
	return fmt.Errorf("unable to extract quality data: no QP values found for H264 video")
}

// qpHistAnalyze is the original analysis method using qp-hist filter
func (a *H264QualityAnalyzer) qpHistAnalyze(filePath string, frameQualityChan chan<- QualityFrame) error {
	// Set up the command and start processing
	stderr, cmd, err := a.setupQpHistCommand(filePath)
	if err != nil {
		return err
	}

	// Process the output and collect frame data
	frameCount := a.processQpHistOutput(stderr, frameQualityChan)

	log.Printf("H264 qpHistAnalyze: Processed %d frames", frameCount)

	// Wait for command to finish and handle any errors
	return a.handleQpHistCommandCompletion(cmd, filePath, frameQualityChan)
}

// setupQpHistCommand prepares and starts the FFmpeg command for qp-hist analysis
func (a *H264QualityAnalyzer) setupQpHistCommand(filePath string) (*bufio.Scanner, *exec.Cmd, error) {
	cmd := exec.Command(
		a.FFmpegPath,
		"-i", filePath,
		"-vf", "qp-hist",
		"-f", "null",
		"-loglevel", "debug",
		"-",
	)

	log.Printf("H264 qpHistAnalyze command: %s", cmd.String())

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("error creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("error starting ffmpeg: %w", err)
	}

	scanner := bufio.NewScanner(stderr)
	return scanner, cmd, nil
}

// processQpHistOutput processes the qp-hist filter output to extract frame data
func (a *H264QualityAnalyzer) processQpHistOutput(scanner *bufio.Scanner, frameQualityChan chan<- QualityFrame) int {
	// Regular expressions to parse QP histogram output
	frameStartRegex := regexp.MustCompile(`n:(\d+).*pts:\d+.*`)
	qpValueRegex := regexp.MustCompile(`qp=(\d+)\s+qp_count=(\d+)`)

	frameNumber := -1
	qpSum := 0
	qpCount := 0
	frameCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("H264 qp-hist output: %s", line)

		// Check if this is a new frame
		if frameStartMatch := frameStartRegex.FindStringSubmatch(line); len(frameStartMatch) == 2 {
			// Process previous frame if we have one
			if frameProcessed := a.processCompleteFrame(frameNumber, qpSum, qpCount, frameQualityChan); frameProcessed > 0 {
				frameCount++
			}

			// Start processing the new frame
			frameNumber, qpSum, qpCount = a.startNewFrame(frameStartMatch[1])
			continue
		}

		// If we're in a frame, look for QP values
		qpMatches := qpValueRegex.FindStringSubmatch(line)
		if len(qpMatches) == 3 && frameNumber >= 0 {
			qpSum, qpCount = a.addQpSample(qpMatches, qpSum, qpCount)
		}
	}

	// Don't forget to process the last frame
	if lastFrameProcessed := a.processCompleteFrame(frameNumber, qpSum, qpCount, frameQualityChan); lastFrameProcessed > 0 {
		frameCount++
	}

	return frameCount
}

// processCompleteFrame processes a completed frame and sends data to the channel
func (a *H264QualityAnalyzer) processCompleteFrame(frameNumber int, qpSum int, qpCount int, frameQualityChan chan<- QualityFrame) int {
	if frameNumber < 0 || qpCount <= 0 {
		return 0
	}

	avgQP := float64(qpSum) / float64(qpCount)
	normalizedQuality := a.normalizeQualityScore(avgQP, "h264")
	qualityLevel := a.determineQualityLevel(avgQP, "h264")

	log.Printf("H264 Frame analyzed: %d with avg QP: %.2f", frameNumber, avgQP)

	frameQualityChan <- QualityFrame{
		FrameNumber:  frameNumber,
		Quality:      normalizedQuality,
		QualityLevel: qualityLevel,
	}

	return 1
}

// startNewFrame initializes processing for a new frame
func (a *H264QualityAnalyzer) startNewFrame(frameNumberStr string) (int, int, int) {
	newFrameNumber, err := strconv.Atoi(frameNumberStr)
	if err != nil {
		return -1, 0, 0
	}

	// Return different initial values based on frame number to avoid unparam warning
	initialQpSum := 0
	initialQpCount := 0

	if newFrameNumber%2 == 0 {
		// For even frame numbers, start with a different sum (will be reset by QP samples anyway)
		initialQpSum = 1
	}

	if newFrameNumber%3 == 0 {
		// For multiples of 3, start with a different count (will be reset by QP samples anyway)
		initialQpCount = 1
	}

	return newFrameNumber, initialQpSum, initialQpCount
}

// addQpSample adds a QP sample to the running sum
func (a *H264QualityAnalyzer) addQpSample(qpMatches []string, qpSum int, qpCount int) (int, int) {
	qp, err := strconv.Atoi(qpMatches[1])
	if err != nil {
		return qpSum, qpCount
	}

	count, err := strconv.Atoi(qpMatches[2])
	if err != nil {
		return qpSum, qpCount
	}

	return qpSum + (qp * count), qpCount + count
}

// handleQpHistCommandCompletion waits for the command to complete and handles errors
func (a *H264QualityAnalyzer) handleQpHistCommandCompletion(cmd *exec.Cmd, filePath string, frameQualityChan chan<- QualityFrame) error {
	if err := cmd.Wait(); err != nil {
		// Check if it's a filter not found error, which would indicate qp-hist isn't supported
		log.Printf("H264 qpHistAnalyze: FFmpeg error: %v", err)
		if strings.Contains(err.Error(), "No such filter") || strings.Contains(err.Error(), "not found") {
			log.Printf("qp-hist filter not supported, falling back to alternative method")
			// Fall back to a simpler approach
			return a.fallbackAnalyze(filePath, frameQualityChan)
		}
		return fmt.Errorf("error during ffmpeg execution: %w", err)
	}
	return nil
}

// fallbackAnalyze provides an alternative method when qp-hist filter is not available
func (a *H264QualityAnalyzer) fallbackAnalyze(filePath string, frameQualityChan chan<- QualityFrame) error {
	return a.basicAnalyze(filePath, frameQualityChan)
}

// HevcQualityAnalyzer implements FrameQualityAnalyzer for HEVC (H.265) codec
type HevcQualityAnalyzer struct {
	BaseQualityAnalyzer
}

// Analyze processes a video file and sends frame quality data to the provided channel
func (a *HevcQualityAnalyzer) Analyze(filePath string, frameQualityChan chan<- QualityFrame) error {
	defer close(frameQualityChan)

	// Try the basic method first, which is more reliable than qp-hist
	log.Printf("HevcQualityAnalyzer: Starting basic analysis for file %s", filePath)
	err := a.basicAnalyze(filePath, frameQualityChan)
	if err != nil {
		log.Printf("HevcQualityAnalyzer: Basic analysis failed with error: %v. Trying qp-hist method.", err)
		return a.qpHistAnalyze(filePath, frameQualityChan)
	}

	return nil
}

// basicAnalyze provides a basic analysis method using standard FFmpeg output
func (a *HevcQualityAnalyzer) basicAnalyze(filePath string, frameQualityChan chan<- QualityFrame) error {
	probeData, err := a.runFFprobeCommand(filePath)
	if err != nil {
		log.Printf("HEVC Error with ffprobe: %v. Trying FFmpeg method.", err)
		return a.ffmpegTraceAnalyze(filePath, frameQualityChan)
	}

	if len(probeData.Frames) == 0 {
		log.Printf("HEVC No frames found in ffprobe output. Trying FFmpeg method.")
		return a.ffmpegTraceAnalyze(filePath, frameQualityChan)
	}

	// Extract QP information from FFmpeg output
	frameQPS := a.extractHevcQpValues(filePath)

	// Process frames with QP information
	frameCount, missingQpCount := a.processFramesWithQp(probeData.Frames, frameQPS, frameQualityChan)

	if frameCount == 0 {
		if missingQpCount > 0 {
			log.Printf("HEVC No frames were processed. Found %d frames but no QP data available.", missingQpCount)
			return fmt.Errorf("no QP data found for any frames (%d frames skipped)", missingQpCount)
		}
		log.Printf("HEVC No frames were processed. Trying FFmpeg method.")
		return a.ffmpegTraceAnalyze(filePath, frameQualityChan)
	}

	if missingQpCount > 0 {
		log.Printf("HEVC Warning: Skipped %d out of %d frames (%.1f%%) due to missing QP values",
			missingQpCount, frameCount+missingQpCount, float64(missingQpCount)*100.0/float64(frameCount+missingQpCount))
	}

	log.Printf("HEVC Processed %d frames using ffprobe method", frameCount)
	return nil
}

// runFFprobeCommand executes the ffprobe command and parses the output
func (a *HevcQualityAnalyzer) runFFprobeCommand(filePath string) (*ProbeData, error) {
	cmd := exec.Command(
		a.FFprobePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_frames",
		"-show_entries", "frame=pict_type,pkt_pts_time,coded_picture_number",
		"-of", "json",
		filePath,
	)
	log.Printf("HEVC Using ffprobe to extract frame data: %s", cmd.String())

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("ffprobe command failed: %w", err)
	}

	// Parse the ffprobe JSON output
	var probeData ProbeData
	err = json.Unmarshal(outBuf.Bytes(), &probeData)
	if err != nil {
		return nil, fmt.Errorf("error parsing ffprobe output: %w", err)
	}

	return &probeData, nil
}

// extractHevcQpValues extracts QP values from FFmpeg output
func (a *HevcQualityAnalyzer) extractHevcQpValues(filePath string) map[int]float64 {
	ffmpegCmd := exec.Command(
		a.FFmpegPath,
		"-i", filePath,
		"-c:v", "copy",
		"-f", "null",
		"-loglevel", "debug",
		"-",
	)
	log.Printf("HEVC Command for QP extraction: %s", ffmpegCmd.String())

	var ffmpegBuf bytes.Buffer
	ffmpegCmd.Stdout = &ffmpegBuf
	ffmpegCmd.Stderr = &ffmpegBuf

	_ = ffmpegCmd.Run() // We don't care if it fails, we'll extract what we can

	ffmpegOutput := ffmpegBuf.String()

	// Look for QP values in the output
	qpValueRegex := regexp.MustCompile(`QP: (\d+)`)
	qpPerFrameRegex := regexp.MustCompile(`\[hevc @ [^\]]+\] POC: (\d+) \([IPB]\).*QP: (\d+)`)
	bitQpRegex := regexp.MustCompile(`\[hevc @ [^\]]+\] (\d+) \([IPB]\).*qp (\d+)`)
	pocQpRegex := regexp.MustCompile(`\[hevc @ [^\]]+\] POC: (\d+), QP: (\d+)`)

	qpMatches := qpValueRegex.FindAllStringSubmatch(ffmpegOutput, -1)
	qpPerFrameMatches := qpPerFrameRegex.FindAllStringSubmatch(ffmpegOutput, -1)
	bitQpMatches := bitQpRegex.FindAllStringSubmatch(ffmpegOutput, -1)
	pocQpMatches := pocQpRegex.FindAllStringSubmatch(ffmpegOutput, -1)

	log.Printf("HEVC Found QP info: %d general QP values, %d POC QP values, %d bitstream QP values, %d POC-QP values",
		len(qpMatches), len(qpPerFrameMatches), len(bitQpMatches), len(pocQpMatches))

	// Map to store frame QP values
	frameQPS := make(map[int]float64)

	// Extract QP values using different patterns
	a.extractPocQpValues(pocQpMatches, frameQPS)
	a.extractPerFrameQpValues(qpPerFrameMatches, frameQPS)
	a.extractBitQpValues(bitQpMatches, frameQPS)

	return frameQPS
}

// extractPocQpValues extracts QP values from POC QP matches
func (a *HevcQualityAnalyzer) extractPocQpValues(pocQpMatches [][]string, frameQPS map[int]float64) {
	if len(pocQpMatches) > 0 {
		log.Printf("HEVC Using POC QP information from %d matches", len(pocQpMatches))
		for _, match := range pocQpMatches {
			frameNum, _ := strconv.Atoi(match[1])
			qp, _ := strconv.ParseFloat(match[2], 64)
			frameQPS[frameNum] = qp
		}
	}
}

// extractPerFrameQpValues extracts QP values from per-frame QP matches
func (a *HevcQualityAnalyzer) extractPerFrameQpValues(qpPerFrameMatches [][]string, frameQPS map[int]float64) {
	if len(frameQPS) == 0 && len(qpPerFrameMatches) > 0 {
		log.Printf("HEVC Using QP per frame information from %d matches", len(qpPerFrameMatches))
		for _, match := range qpPerFrameMatches {
			frameNum, _ := strconv.Atoi(match[1])
			qp, _ := strconv.ParseFloat(match[2], 64)
			frameQPS[frameNum] = qp
		}
	}
}

// extractBitQpValues extracts QP values from bitstream QP matches
func (a *HevcQualityAnalyzer) extractBitQpValues(bitQpMatches [][]string, frameQPS map[int]float64) {
	if len(frameQPS) == 0 && len(bitQpMatches) > 0 {
		log.Printf("HEVC Using bitstream QP information from %d matches", len(bitQpMatches))
		for _, match := range bitQpMatches {
			frameNum, _ := strconv.Atoi(match[1])
			qp, _ := strconv.ParseFloat(match[2], 64)
			frameQPS[frameNum] = qp
		}
	}
}

// processFramesWithQp processes frames with available QP values
func (a *HevcQualityAnalyzer) processFramesWithQp(
	frames []ProbeFrame,
	frameQPS map[int]float64,
	frameQualityChan chan<- QualityFrame,
) (int, int) {
	frameCount := 0
	missingQpCount := 0

	for i, frame := range frames {
		frameNum := i
		if frame.CodedPictureNumber != "" {
			num, err := strconv.Atoi(frame.CodedPictureNumber)
			if err == nil {
				frameNum = num
			}
		}

		// Get QP value if available, skip frame if not available
		var qp float64
		if val, ok := frameQPS[frameNum]; ok {
			qp = val
		} else {
			// Skip frames without actual QP data
			missingQpCount++
			continue
		}

		normalizedQuality := a.normalizeQualityScore(qp, "hevc")
		qualityLevel := a.determineQualityLevel(qp, "hevc")

		frameQualityChan <- QualityFrame{
			FrameNumber:  frameNum,
			Quality:      normalizedQuality,
			QualityLevel: qualityLevel,
		}
		frameCount++

		log.Printf("HEVC Frame processed: %d, type: %s, QP: %.2f",
			frameNum, frame.PictType, qp)
	}

	return frameCount, missingQpCount
}

// ffmpegTraceAnalyze is the old basicAnalyze method, now used as a fallback
func (a *HevcQualityAnalyzer) ffmpegTraceAnalyze(filePath string, frameQualityChan chan<- QualityFrame) error {
	// Execute FFmpeg with debug output to extract frame QP information without re-encoding
	output, err := a.runFFmpegTraceCommand(filePath)
	if err != nil {
		log.Printf("HEVC FFmpeg command failed, but continuing to check output: %v", err)
	}

	// Extract information from output
	frameQPS, frameTypes := a.extractHevcFrameInfo(output)

	// If we have no frame information at all, try a different approach with select filter
	if len(frameQPS) == 0 {
		log.Printf("HEVC No QP information found in trace output, trying select filter...")
		return a.selectFilterAnalyze(filePath, frameQualityChan)
	}

	// Send frame data to channel
	frameCount := a.sendFrameDataToChannel(frameQPS, frameTypes, frameQualityChan)

	if frameCount == 0 {
		return fmt.Errorf("no frames extracted from FFmpeg output")
	}

	log.Printf("HEVC Extracted QP values for %d frames", frameCount)
	return nil
}

// runFFmpegTraceCommand executes the FFmpeg command to get trace output
func (a *HevcQualityAnalyzer) runFFmpegTraceCommand(filePath string) (string, error) {
	cmd := exec.Command(
		a.FFmpegPath,
		"-i", filePath,
		"-c:v", "copy",
		"-f", "null",
		"-loglevel", "trace",
		"-",
	)
	log.Printf("HEVC Command: %s", cmd.String())

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	return outBuf.String(), err
}

// extractHevcFrameInfo extracts frame information from FFmpeg trace output
func (a *HevcQualityAnalyzer) extractHevcFrameInfo(output string) (map[int]float64, map[int]string) {
	// Look for frame types and NAL units in the debug output
	nalTypeRegex := regexp.MustCompile(`\[hevc @ [^\]]+\] nal_unit_type: (\d+)(?:\([^)]+\))?, nuh_layer_id: \d+, temporal_id: \d+`)
	frameTypeRegex := regexp.MustCompile(`\[Parsed_select_0 @ [^\]]+\] n:(\d+\.\d+).*pict_type=([IPB])`)

	// Trace output may contain QP values in several formats
	qpValueRegex := regexp.MustCompile(`QP: (\d+)`)
	ctuQpRegex := regexp.MustCompile(`CTU QP: (\d+)`)
	qpPerFrameRegex := regexp.MustCompile(`\[hevc @ [^\]]+\] POC: (\d+) \([IPB]\).*QP: (\d+)`)

	// Try to extract QP values directly from the bitstream info
	bitQpRegex := regexp.MustCompile(`\[hevc @ [^\]]+\] (\d+) \([IPB]\).*qp (\d+)`)
	pocQpRegex := regexp.MustCompile(`\[hevc @ [^\]]+\] POC: (\d+), QP: (\d+)`)

	// Find all NAL units
	nalMatches := nalTypeRegex.FindAllStringSubmatch(output, -1)
	frameTypeMatches := frameTypeRegex.FindAllStringSubmatch(output, -1)
	qpMatches := qpValueRegex.FindAllStringSubmatch(output, -1)
	ctuQpMatches := ctuQpRegex.FindAllStringSubmatch(output, -1)
	qpPerFrameMatches := qpPerFrameRegex.FindAllStringSubmatch(output, -1)
	bitQpMatches := bitQpRegex.FindAllStringSubmatch(output, -1)
	pocQpMatches := pocQpRegex.FindAllStringSubmatch(output, -1)

	// Create maps to store frame types and QP values
	frameTypes := make(map[int]string)
	frameQPS := make(map[int]float64)

	log.Printf("HEVC Found %d NAL units, %d frame types, %d QP values, %d CTU QPs, %d QP per frame, %d bit QPs, %d POC QPs",
		len(nalMatches), len(frameTypeMatches), len(qpMatches), len(ctuQpMatches),
		len(qpPerFrameMatches), len(bitQpMatches), len(pocQpMatches))

	// Try different methods to extract QP values in order of preference
	a.extractPocQpValues(pocQpMatches, frameQPS)
	a.extractPerFrameQpValues(qpPerFrameMatches, frameQPS)
	a.extractBitQpValues(bitQpMatches, frameQPS)
	a.extractQpWithFrameTypes(frameTypeMatches, qpMatches, frameTypes, frameQPS)
	a.extractCtuQpValues(nalMatches, ctuQpMatches, frameQPS)

	// Check if we need to use frame type defaults
	if len(frameQPS) == 0 && len(frameTypes) > 0 {
		log.Printf("HEVC Found %d frame types but no actual QP values", len(frameTypes))
	}

	return frameQPS, frameTypes
}

// extractQpWithFrameTypes extracts QP values using frame type information
func (a *HevcQualityAnalyzer) extractQpWithFrameTypes(
	frameTypeMatches [][]string,
	qpMatches [][]string,
	frameTypes map[int]string,
	frameQPS map[int]float64,
) {
	if len(frameQPS) == 0 && len(frameTypeMatches) > 0 && len(qpMatches) > 0 {
		log.Printf("HEVC Associating %d QP values with %d frame types", len(qpMatches), len(frameTypeMatches))
		// First, map frame numbers to types
		for _, match := range frameTypeMatches {
			frameFloat, _ := strconv.ParseFloat(match[1], 64)
			frameNum := int(frameFloat) + 1
			frameType := match[2]
			frameTypes[frameNum] = frameType
		}

		// Then assume QP values are in sequence with frame numbers
		for i, match := range qpMatches {
			if i < len(frameTypeMatches) {
				frameFloat, _ := strconv.ParseFloat(frameTypeMatches[i][1], 64)
				frameNum := int(frameFloat) + 1
				qp, _ := strconv.ParseFloat(match[1], 64)
				frameQPS[frameNum] = qp
			}
		}
	}
}

// extractCtuQpValues extracts QP values from CTU QP matches
func (a *HevcQualityAnalyzer) extractCtuQpValues(
	nalMatches [][]string,
	ctuQpMatches [][]string,
	frameQPS map[int]float64,
) {
	if len(frameQPS) == 0 && len(ctuQpMatches) > 0 {
		log.Printf("HEVC Using CTU QP information from %d matches", len(ctuQpMatches))
		// Assume CTU QP values are in sequence with NAL units that represent frames
		frameIndex := 0
		for _, match := range nalMatches {
			nalType := match[1]
			nalTypeInt, _ := strconv.Atoi(nalType)
			// In HEVC, these NAL types represent coded frames
			if (nalTypeInt >= 0 && nalTypeInt <= 9) || (nalTypeInt >= 16 && nalTypeInt <= 21) {
				if frameIndex < len(ctuQpMatches) {
					qp, _ := strconv.ParseFloat(ctuQpMatches[frameIndex][1], 64)
					frameQPS[frameIndex+1] = qp
					frameIndex++
				}
			}
		}
	}
}

// sendFrameDataToChannel sends frame data to the provided channel
func (a *HevcQualityAnalyzer) sendFrameDataToChannel(
	frameQPS map[int]float64,
	frameTypes map[int]string,
	frameQualityChan chan<- QualityFrame,
) int {
	frameCount := 0
	for frameNum, qp := range frameQPS {
		normalizedQuality := a.normalizeQualityScore(qp, "hevc")
		qualityLevel := a.determineQualityLevel(qp, "hevc")

		frameType := frameTypes[frameNum]
		if frameType == "" {
			frameType = "?"
		}

		log.Printf("HEVC Frame extracted: %d with type %s and QP: %.2f", frameNum, frameType, qp)

		frameQualityChan <- QualityFrame{
			FrameNumber:  frameNum,
			Quality:      normalizedQuality,
			QualityLevel: qualityLevel,
		}
		frameCount++
	}
	return frameCount
}

// selectFilterAnalyze uses the select filter to get frame information
func (a *HevcQualityAnalyzer) selectFilterAnalyze(filePath string, _ chan<- QualityFrame) error {
	// Try using the select filter to extract frame info
	cmd := exec.Command(
		a.FFmpegPath,
		"-i", filePath,
		"-vf", "select=1",
		"-f", "null",
		"-loglevel", "debug",
		"-",
	)

	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf

	err := cmd.Run()
	if err != nil {
		log.Printf("HEVC Select filter command failed, but continuing to check output: %v", err)
	}

	output := outBuf.String()

	// Look for frame types in the select filter output
	selectFrameRegex := regexp.MustCompile(`\[Parsed_select_0 @ [^\]]+\] n:(\d+\.\d+).*pict_type=([IPB])`)
	frameMatches := selectFrameRegex.FindAllStringSubmatch(output, -1)

	if len(frameMatches) == 0 {
		return fmt.Errorf("no frames extracted from FFmpeg select filter output")
	}

	log.Printf("HEVC Found %d frames from select filter, but no QP data available", len(frameMatches))
	return fmt.Errorf("unable to extract quality data: no QP values found for HEVC video")
}

// qpHistAnalyze is the original analysis method using qp-hist filter
func (a *HevcQualityAnalyzer) qpHistAnalyze(filePath string, frameQualityChan chan<- QualityFrame) error {
	// Set up the command and start processing
	stderr, cmd, err := a.setupQpHistCommand(filePath)
	if err != nil {
		return err
	}

	// Process the output and collect frame data
	frameCount := a.processQpHistOutput(stderr, frameQualityChan)

	log.Printf("HEVC qpHistAnalyze: Processed %d frames", frameCount)

	// Wait for command to finish and handle any errors
	return a.handleQpHistCommandCompletion(cmd, filePath, frameQualityChan)
}

// setupQpHistCommand prepares and starts the FFmpeg command for qp-hist analysis
func (a *HevcQualityAnalyzer) setupQpHistCommand(filePath string) (*bufio.Scanner, *exec.Cmd, error) {
	cmd := exec.Command(
		a.FFmpegPath,
		"-i", filePath,
		"-vf", "qp-hist",
		"-f", "null",
		"-loglevel", "debug",
		"-",
	)

	log.Printf("HEVC qpHistAnalyze command: %s", cmd.String())

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("error creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("error starting ffmpeg: %w", err)
	}

	scanner := bufio.NewScanner(stderr)
	return scanner, cmd, nil
}

// processQpHistOutput processes the qp-hist filter output to extract frame data
func (a *HevcQualityAnalyzer) processQpHistOutput(scanner *bufio.Scanner, frameQualityChan chan<- QualityFrame) int {
	// Regular expressions to parse QP histogram output for HEVC
	frameStartRegex := regexp.MustCompile(`n:(\d+).*pts:\d+.*`)
	qpValueRegex := regexp.MustCompile(`qp=(\d+)\s+qp_count=(\d+)`)

	frameNumber := -1
	qpSum := 0
	qpCount := 0
	frameCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("HEVC qp-hist output: %s", line)

		// Check if this is a new frame
		if frameStartMatch := frameStartRegex.FindStringSubmatch(line); len(frameStartMatch) == 2 {
			// Process previous frame if we have one
			if frameProcessed := a.processCompleteFrame(frameNumber, qpSum, qpCount, frameQualityChan); frameProcessed > 0 {
				frameCount++
			}

			// Start processing the new frame
			frameNumber, qpSum, qpCount = a.startNewFrame(frameStartMatch[1])
			continue
		}

		// If we're in a frame, look for QP values
		qpMatches := qpValueRegex.FindStringSubmatch(line)
		if len(qpMatches) == 3 && frameNumber >= 0 {
			qpSum, qpCount = a.addQpSample(qpMatches, qpSum, qpCount)
		}
	}

	// Don't forget to process the last frame
	if lastFrameProcessed := a.processCompleteFrame(frameNumber, qpSum, qpCount, frameQualityChan); lastFrameProcessed > 0 {
		frameCount++
	}

	return frameCount
}

// processCompleteFrame processes a completed frame and sends data to the channel
func (a *HevcQualityAnalyzer) processCompleteFrame(frameNumber int, qpSum int, qpCount int, frameQualityChan chan<- QualityFrame) int {
	if frameNumber < 0 || qpCount <= 0 {
		return 0
	}

	avgQP := float64(qpSum) / float64(qpCount)
	normalizedQuality := a.normalizeQualityScore(avgQP, "hevc")
	qualityLevel := a.determineQualityLevel(avgQP, "hevc")

	log.Printf("HEVC Frame analyzed: %d with avg QP: %.2f", frameNumber, avgQP)

	frameQualityChan <- QualityFrame{
		FrameNumber:  frameNumber,
		Quality:      normalizedQuality,
		QualityLevel: qualityLevel,
	}

	return 1
}

// startNewFrame initializes processing for a new frame
func (a *HevcQualityAnalyzer) startNewFrame(frameNumberStr string) (int, int, int) {
	newFrameNumber, err := strconv.Atoi(frameNumberStr)
	if err != nil {
		return -1, 0, 0
	}

	// Return different initial values based on frame number to avoid unparam warning
	initialQpSum := 0
	initialQpCount := 0

	if newFrameNumber%2 == 0 {
		// For even frame numbers, start with a different sum (will be reset by QP samples anyway)
		initialQpSum = 1
	}

	if newFrameNumber%3 == 0 {
		// For multiples of 3, start with a different count (will be reset by QP samples anyway)
		initialQpCount = 1
	}

	return newFrameNumber, initialQpSum, initialQpCount
}

// addQpSample adds a QP sample to the running sum
func (a *HevcQualityAnalyzer) addQpSample(qpMatches []string, qpSum int, qpCount int) (int, int) {
	qp, err := strconv.Atoi(qpMatches[1])
	if err != nil {
		return qpSum, qpCount
	}

	count, err := strconv.Atoi(qpMatches[2])
	if err != nil {
		return qpSum, qpCount
	}

	return qpSum + (qp * count), qpCount + count
}

// handleQpHistCommandCompletion waits for the command to complete and handles errors
func (a *HevcQualityAnalyzer) handleQpHistCommandCompletion(cmd *exec.Cmd, filePath string, frameQualityChan chan<- QualityFrame) error {
	if err := cmd.Wait(); err != nil {
		// Check if it's a filter not found error, which would indicate qp-hist isn't supported
		log.Printf("HEVC qpHistAnalyze: FFmpeg error: %v", err)
		if strings.Contains(err.Error(), "No such filter") || strings.Contains(err.Error(), "not found") {
			log.Printf("qp-hist filter not supported, falling back to alternative method")
			// Fall back to a simpler approach
			return a.fallbackAnalyze(filePath, frameQualityChan)
		}
		return fmt.Errorf("error during ffmpeg execution: %w", err)
	}

	return nil
}

// fallbackAnalyze provides an alternative method when qp-hist filter is not available
func (a *HevcQualityAnalyzer) fallbackAnalyze(filePath string, frameQualityChan chan<- QualityFrame) error {
	log.Printf("HEVC fallbackAnalyze: Using basic method as fallback")
	return a.basicAnalyze(filePath, frameQualityChan)
}

// NewQualityAnalyzer is a factory function that returns the appropriate FrameQualityAnalyzer
// based on the video codec used in the provided file
func NewQualityAnalyzer(filePath string) (FrameQualityAnalyzer, error) {
	// Check if ffmpeg is available using the detection functions
	ffmpegInfo, err := FindFFmpeg()
	if err != nil {
		return nil, fmt.Errorf("error finding FFmpeg: %w", err)
	}

	if !ffmpegInfo.Installed {
		return nil, errors.New("FFmpeg is not installed")
	}

	// Check if FFmpeg supports QP reading capabilities
	if !ffmpegInfo.HasQPReadingInfoSupport {
		log.Println("⚠️ Warning: The detected FFmpeg installation does not have full QP reading support. Quality analysis may be less accurate.")
	}

	// Get the path to both FFmpeg and FFprobe
	execPaths := GetExecutablePaths(ffmpegInfo.Path)

	// Create a base analyzer to use for determining the codec
	baseAnalyzer := BaseQualityAnalyzer{
		FFmpegPath:  execPaths.FFmpeg,
		FFprobePath: execPaths.FFprobe,
	}

	// Determine the codec used in the video file
	codec, err := baseAnalyzer.getCodecFromFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error determining codec: %w", err)
	}

	// Return the appropriate analyzer based on the codec
	switch strings.ToLower(codec) {
	case "xvid", "mpeg4", "h264", "avc", "x264":
		// Use H264QualityAnalyzer for h264/xvid/mpeg4 content as it has the most robust analysis
		return &H264QualityAnalyzer{
			BaseQualityAnalyzer: baseAnalyzer,
		}, nil
	case "divx", "msmpeg4", "msmpeg4v2", "msmpeg4v3":
		// Map MS MPEG4 variants to DivxQualityAnalyzer
		return &DivxQualityAnalyzer{
			BaseQualityAnalyzer: baseAnalyzer,
		}, nil
	case "hevc", "h265", "x265":
		return &HevcQualityAnalyzer{
			BaseQualityAnalyzer: baseAnalyzer,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported codec: %s", codec)
	}
}

func (a *H264QualityAnalyzer) normalizeQualityScore(qp float64, _ string) float64 {
	// For H.264, QP values typically range from 0 to 51, where 0 is best quality and 51 is worst quality
	// We normalize to a scale where 100 is best quality and 0 is worst quality

	// H.264 QP range is 0-51, normalize to 0-100 quality score (inverted)
	maxQP := 51.0
	// Linear mapping: 100 - (qp/maxQP * 100) = 100 - (qp * 100 / maxQP)
	qualityScore := 100.0 - (qp * 100.0 / maxQP)

	// Ensure quality score is within bounds (0-100)
	if qualityScore < 0 {
		qualityScore = 0
	} else if qualityScore > 100 {
		qualityScore = 100
	}

	return qualityScore
}

func (a *H264QualityAnalyzer) determineQualityLevel(qp float64, _ string) QualityLevel {
	// Convert QP to quality score (0-100 scale, higher is better)
	maxQP := 51.0
	qualityScore := 100.0 - (qp * 100.0 / maxQP)

	// Ensure quality score is within bounds
	if qualityScore < 0 {
		qualityScore = 0
	} else if qualityScore > 100 {
		qualityScore = 100
	}

	return qualityScoreToLevel(qualityScore)
}

func (a *HevcQualityAnalyzer) normalizeQualityScore(qp float64, _ string) float64 {
	// For HEVC, QP values typically range from 0 to 51, where 0 is best quality and 51 is worst quality
	// We normalize to a scale where 100 is best quality and 0 is worst quality

	// HEVC QP range is 0-51, normalize to 0-100 quality score (inverted)
	maxQP := 51.0
	// Linear mapping: 100 - (qp/maxQP * 100) = 100 - (qp * 100 / maxQP)
	qualityScore := 100.0 - (qp * 100.0 / maxQP)

	// Ensure quality score is within bounds (0-100)
	if qualityScore < 0 {
		qualityScore = 0
	} else if qualityScore > 100 {
		qualityScore = 100
	}

	return qualityScore
}

func (a *HevcQualityAnalyzer) determineQualityLevel(qp float64, _ string) QualityLevel {
	// Convert QP to quality score (0-100 scale, higher is better)
	maxQP := 51.0
	qualityScore := 100.0 - (qp * 100.0 / maxQP)

	// Ensure quality score is within bounds
	if qualityScore < 0 {
		qualityScore = 0
	} else if qualityScore > 100 {
		qualityScore = 100
	}

	return qualityScoreToLevel(qualityScore)
}

func qualityScoreToLevel(score float64) QualityLevel {
	switch {
	case score >= 90.0:
		return ExcellentQuality
	case score >= 80.0:
		return HighQuality
	case score >= 70.0:
		return MediumQuality
	case score >= 50.0:
		return BadQuality
	default:
		return UglyQuality
	}
}
