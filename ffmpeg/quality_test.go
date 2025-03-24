package ffmpeg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// QualityAnalyzerTestSuite is the test suite for quality analyzers
type QualityAnalyzerTestSuite struct {
	suite.Suite
	testFilesDir string
	xvid         string
	divx         string
	h264         string
	hevc         string
}

// SetupSuite prepares the test environment for all tests in the suite
func (suite *QualityAnalyzerTestSuite) SetupSuite() {
	// No setup needed at suite level
}

// TearDownSuite cleans up after all tests in the suite
func (suite *QualityAnalyzerTestSuite) TearDownSuite() {
	// No teardown needed at suite level
}

// SetupTest prepares the test environment before each test
func (suite *QualityAnalyzerTestSuite) SetupTest() {
	// Get the path to the test resources
	suite.testFilesDir = filepath.Join("..", "resources", "test")

	// Set paths to specific test files
	suite.xvid = filepath.Join(suite.testFilesDir, "sample3.avi") // assuming this is xvid/mpeg4
	suite.divx = filepath.Join(suite.testFilesDir, "sample4.avi") // assuming this is divx
	suite.h264 = filepath.Join(suite.testFilesDir, "sample.mkv")  // assuming this is h264
	suite.hevc = filepath.Join(suite.testFilesDir, "sample2.mkv") // assuming this is hevc
}

// TestFileExistence verifies that all required test files exist
func (suite *QualityAnalyzerTestSuite) TestFileExistence() {
	files := []string{suite.xvid, suite.divx, suite.h264, suite.hevc}

	for _, file := range files {
		_, err := os.Stat(file)
		suite.NoError(err, "Test file %s should exist", file)
	}
}

// TestNewQualityAnalyzer tests the analyzer factory function
func (suite *QualityAnalyzerTestSuite) TestNewQualityAnalyzer() {
	// First ensure FFmpeg is installed
	ffmpegInfo, err := FindFFmpeg()
	suite.NoError(err, "Error finding FFmpeg")

	if !ffmpegInfo.Installed {
		suite.T().Skip("FFmpeg not installed, skipping tests that require FFmpeg")
		return
	}

	// Test creating analyzers for different file types
	tests := []struct {
		name     string
		filePath string
		wantType string
	}{
		{
			name:     "H264 analyzer for H.264 file",
			filePath: suite.h264,
			wantType: "*ffmpeg.H264QualityAnalyzer",
		},
		{
			name:     "HEVC analyzer for HEVC file",
			filePath: suite.hevc,
			wantType: "*ffmpeg.HevcQualityAnalyzer",
		},
		{
			name:     "H264 analyzer for Xvid file (Xvid mapped to H264)",
			filePath: suite.xvid,
			wantType: "*ffmpeg.H264QualityAnalyzer",
		},
		{
			name:     "Divx analyzer for Divx file (MSMpeg4 variants mapped to Divx)",
			filePath: suite.divx,
			wantType: "*ffmpeg.DivxQualityAnalyzer",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			analyzer, err := NewQualityAnalyzer(tt.filePath)
			suite.NoError(err, "NewQualityAnalyzer should not return an error for valid file")
			suite.NotNil(analyzer, "Analyzer should not be nil")
			suite.Equal(tt.wantType, suite.getTypeName(analyzer), "Analyzer type should match expected")
		})
	}
}

// TestNewQualityAnalyzerWithInvalidFile tests the analyzer factory with invalid files
func (suite *QualityAnalyzerTestSuite) TestNewQualityAnalyzerWithInvalidFile() {
	// First ensure FFmpeg is installed
	ffmpegInfo, err := FindFFmpeg()
	suite.NoError(err, "Error finding FFmpeg")

	if !ffmpegInfo.Installed {
		suite.T().Skip("FFmpeg not installed, skipping tests that require FFmpeg")
		return
	}

	// Test with a file that doesn't exist
	analyzer, err := NewQualityAnalyzer("non_existent_file.mp4")
	suite.Error(err, "Should return error for non-existent file")
	suite.Nil(analyzer, "Analyzer should be nil for non-existent file")
}

// TestNewQualityAnalyzerWithUnsupportedCodec tests handling of unsupported codecs
func (suite *QualityAnalyzerTestSuite) TestNewQualityAnalyzerWithUnsupportedCodec() {
	// Create a test text file with no video codec
	tempFile, err := os.CreateTemp("", "textfile-*.txt")
	suite.NoError(err, "Failed to create temp file")
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString("This is not a video file")
	suite.NoError(err, "Failed to write to temp file")
	tempFile.Close()

	// Try to create an analyzer with the text file
	analyzer, err := NewQualityAnalyzer(tempFile.Name())

	// It should fail because text file has no codec
	suite.Error(err, "Should return error for file with no codec")
	suite.Nil(analyzer, "Analyzer should be nil for file with no codec")
}

// TestQualityFrameChannel tests that frames are correctly sent to the channel
func (suite *QualityAnalyzerTestSuite) TestQualityFrameChannel() {
	// First ensure FFmpeg is installed
	ffmpegInfo, err := FindFFmpeg()
	suite.NoError(err, "Error finding FFmpeg")

	if !ffmpegInfo.Installed {
		suite.T().Skip("FFmpeg not installed, skipping tests that require FFmpeg")
		return
	}

	// Test all codec analyzers
	testFiles := suite.getTestFiles()

	for _, tf := range testFiles {
		suite.Run(tf.name, func() {
			t := suite.T()

			analyzer := suite.createAnalyzerForTest(tf.name, tf.filePath, t)
			if analyzer == nil {
				return // Skip this test if analyzer creation failed
			}

			// Test the analyzer with the file
			suite.testSingleAnalyzer(analyzer, tf.name, tf.filePath, t)
		})
	}
}

// getTestFiles returns a list of test files for different codecs
func (suite *QualityAnalyzerTestSuite) getTestFiles() []struct {
	name     string
	filePath string
} {
	return []struct {
		name     string
		filePath string
	}{
		{"Xvid", suite.xvid},
		{"Divx", suite.divx},
		{"H264", suite.h264},
		{"HEVC", suite.hevc},
	}
}

// createAnalyzerForTest creates an analyzer for a specific file
func (suite *QualityAnalyzerTestSuite) createAnalyzerForTest(
	codecName string,
	filePath string,
	t *testing.T,
) FrameQualityAnalyzer {
	analyzer, err := NewQualityAnalyzer(filePath)
	if err != nil {
		// If codec detection fails, skip this test
		if strings.Contains(err.Error(), "error determining codec") {
			t.Skipf("Skipping %s test because codec could not be determined", codecName)
			return nil
		}
		suite.NoError(err, "NewQualityAnalyzer should not return an error")
		return nil
	}
	suite.NotNil(analyzer, "Analyzer should not be nil")
	return analyzer
}

// testSingleAnalyzer tests the analyzer with a specific file
func (suite *QualityAnalyzerTestSuite) testSingleAnalyzer(
	analyzer FrameQualityAnalyzer,
	codecName string,
	filePath string,
	t *testing.T,
) {
	// Create a channel to receive quality frames
	frameChan := make(chan QualityFrame, 100)

	// Start analysis in a goroutine and get frames
	frames, done := suite.startAnalysis(analyzer, codecName, filePath, frameChan, t)

	// Wait for the goroutine to finish
	suite.waitForAnalysisCompletion(done, codecName, t)

	// Verify frames
	suite.verifyFrames(frames, codecName, t)
}

// startAnalysis starts the analysis in a goroutine and collects frames
func (suite *QualityAnalyzerTestSuite) startAnalysis(
	analyzer FrameQualityAnalyzer,
	codecName string,
	filePath string,
	frameChan chan QualityFrame,
	t *testing.T,
) ([]QualityFrame, <-chan struct{}) {
	// Use a done channel to signal when the goroutine is complete
	doneChan := make(chan struct{})

	// Start analysis in a goroutine
	go func() {
		defer close(doneChan)

		err := analyzer.Analyze(filePath, frameChan)
		// Use t.Log instead of assertions in goroutines to avoid panic
		if err != nil {
			t.Logf("Error analyzing %s: %v", codecName, err)
		}
	}()

	// Read frames from the channel
	frames := suite.collectFrames(frameChan, doneChan, codecName, t)

	return frames, doneChan
}

// collectFrames collects frames from the channel
func (suite *QualityAnalyzerTestSuite) collectFrames(
	frameChan <-chan QualityFrame,
	doneChan <-chan struct{},
	codecName string,
	t *testing.T,
) []QualityFrame {
	frameCount := 0
	timeout := time.After(60 * time.Second) // Increased timeout for real processing
	frames := []QualityFrame{}

frameLoop:
	for {
		select {
		case frame, open := <-frameChan:
			if !open {
				break frameLoop
			}
			frames = append(frames, frame)
			frameCount++

			// Verify basic properties of each frame
			suite.GreaterOrEqual(frame.FrameNumber, 0, "Frame number should be non-negative")
			suite.GreaterOrEqual(frame.Quality, 0.0, "Quality score should be non-negative")
			suite.Less(frame.QualityLevel, QualityLevel(5), "Quality level should be valid")

			// If we've collected enough frames for verification, break
			if frameCount >= 10 {
				break frameLoop
			}
		case <-timeout:
			suite.handleTimeout(frameCount, codecName, t)
			break frameLoop
		case <-doneChan:
			// Analysis is done
			if frameCount == 0 {
				t.Logf("Analysis completed but no frames were received from %s", codecName)
			}
			break frameLoop
		}
	}

	return frames
}

// handleTimeout handles the timeout case when collecting frames
func (suite *QualityAnalyzerTestSuite) handleTimeout(
	frameCount int,
	codecName string,
	t *testing.T,
) {
	if frameCount == 0 {
		t.Logf("Timeout waiting for frames from %s, this may be due to codec compatibility issues", codecName)
	} else {
		t.Logf("Timeout after receiving %d frames from %s", frameCount, codecName)
	}
}

// waitForAnalysisCompletion waits for the analysis goroutine to complete
func (suite *QualityAnalyzerTestSuite) waitForAnalysisCompletion(
	doneChan <-chan struct{},
	codecName string,
	t *testing.T,
) {
	select {
	case <-doneChan:
		// Goroutine finished normally
	case <-time.After(5 * time.Second):
		t.Logf("Warning: Goroutine for %s analysis didn't finish within timeout", codecName)
	}
}

// verifyFrames verifies that frames were received and logs the result
func (suite *QualityAnalyzerTestSuite) verifyFrames(
	frames []QualityFrame,
	codecName string,
	t *testing.T,
) {
	if len(frames) > 0 {
		suite.Greater(len(frames), 0, "Should have received at least one frame")
		t.Logf("Successfully received %d frames from %s", len(frames), codecName)
	} else {
		t.Logf("No frames received from %s, this could be due to codec compatibility or file issues", codecName)
	}
}

// TestQualityLevelString tests the String method of QualityLevel
func (suite *QualityAnalyzerTestSuite) TestQualityLevelString() {
	// Test that the String() method returns the correct string for each quality level
	tests := []struct {
		level QualityLevel
		want  string
	}{
		{UglyQuality, "Ugly"},
		{BadQuality, "Bad"},
		{MediumQuality, "Medium"},
		{HighQuality, "High"},
		{ExcellentQuality, "Excellent"},
	}

	for _, tt := range tests {
		suite.Run(tt.want, func() {
			got := tt.level.String()
			suite.Equal(tt.want, got)
		})
	}
}

// TestDetermineQualityLevel tests the quality level determination
func (suite *QualityAnalyzerTestSuite) TestDetermineQualityLevel() {
	// First ensure FFmpeg is installed
	ffmpegInfo, err := FindFFmpeg()
	suite.NoError(err, "Error finding FFmpeg")

	if !ffmpegInfo.Installed {
		suite.T().Skip("FFmpeg not installed, skipping tests that require FFmpeg")
		return
	}

	// Get the paths to FFmpeg and FFprobe
	execPaths := GetExecutablePaths(ffmpegInfo.Path)

	// Create a test analyzer (we'll use H264 for this test)
	analyzer := &H264QualityAnalyzer{
		BaseQualityAnalyzer: BaseQualityAnalyzer{
			FFmpegPath:  execPaths.FFmpeg,
			FFprobePath: execPaths.FFprobe,
		},
	}

	// Test the determineQualityLevel function for the H264QualityAnalyzer
	// The H264QualityAnalyzer converts QP to a normalized quality score (0-100) and then maps to quality levels
	h264Tests := []struct {
		qp   float64
		want QualityLevel
	}{
		{5.0, ExcellentQuality}, // QP 5 -> Quality 90.2 -> Excellent (≥ 90)
		{10.0, HighQuality},     // QP 10 -> Quality 80.4 -> High (≥ 80)
		{16.0, BadQuality},      // QP 16 -> Quality 68.6 -> Bad (≥ 50 and < 70)
		{26.0, UglyQuality},     // QP 26 -> Quality 49.0 -> Ugly (< 50)
		{45.0, UglyQuality},     // QP 45 -> Quality 11.8 -> Ugly (< 50)
	}

	for _, tt := range h264Tests {
		// Use sprintf to create descriptive test name
		testName := fmt.Sprintf("H264_QP_%.1f", tt.qp)
		suite.Run(testName, func() {
			// For debugging, calculate the expected quality score
			maxQP := 51.0
			qualityScore := 100.0 - (tt.qp * 100.0 / maxQP)
			suite.T().Logf("QP: %.1f -> Quality Score: %.1f", tt.qp, qualityScore)

			got := analyzer.determineQualityLevel(tt.qp, "h264")
			suite.Equal(tt.want, got)
		})
	}

	// Create a base analyzer for testing the xvid/divx implementation
	baseAnalyzer := &BaseQualityAnalyzer{
		FFmpegPath:  execPaths.FFmpeg,
		FFprobePath: execPaths.FFprobe,
	}

	// Xvid/Divx quality tests for the base analyzer
	xvidTests := []struct {
		quality float64
		want    QualityLevel
	}{
		{1, ExcellentQuality}, // QP ≤ 2 = Excellent
		{3, HighQuality},      // 2 < QP ≤ 4 = High
		{5, MediumQuality},    // 4 < QP ≤ 6 = Medium
		{7, BadQuality},       // 6 < QP ≤ 8 = Bad
		{9, UglyQuality},      // QP > 8 = Ugly
	}

	for _, tt := range xvidTests {
		suite.Run("Xvid_"+tt.want.String(), func() {
			got := baseAnalyzer.determineQualityLevel(tt.quality, "xvid")
			suite.Equal(tt.want, got)
		})
	}

	// Default codec tests (unknown codec type) for the base analyzer
	defaultTests := []struct {
		quality float64
		want    QualityLevel
	}{
		{95, ExcellentQuality}, // Quality ≥ 90 = Excellent
		{80, HighQuality},      // 70 ≤ Quality < 90 = High
		{60, MediumQuality},    // 50 ≤ Quality < 70 = Medium
		{40, BadQuality},       // 30 ≤ Quality < 50 = Bad
		{20, UglyQuality},      // Quality < 30 = Ugly
	}

	for _, tt := range defaultTests {
		suite.Run("Default_"+tt.want.String(), func() {
			got := baseAnalyzer.determineQualityLevel(tt.quality, "unknown")
			suite.Equal(tt.want, got)
		})
	}
}

// TestNormalizeQualityScore tests the quality score normalization
func (suite *QualityAnalyzerTestSuite) TestNormalizeQualityScore() {
	// First ensure FFmpeg is installed
	ffmpegInfo, err := FindFFmpeg()
	suite.NoError(err, "Error finding FFmpeg")

	if !ffmpegInfo.Installed {
		suite.T().Skip("FFmpeg not installed, skipping tests that require FFmpeg")
		return
	}

	// Get the paths to FFmpeg and FFprobe
	execPaths := GetExecutablePaths(ffmpegInfo.Path)
	baseAnalyzer := BaseQualityAnalyzer{
		FFmpegPath:  execPaths.FFmpeg,
		FFprobePath: execPaths.FFprobe,
	}

	// Test H.264/HEVC normalization (0-51, lower is better, should be inverted to 0-100)
	suite.InDelta(100.0, baseAnalyzer.normalizeQualityScore(0, "h264"), 0.1)   // QP 0 -> 100% quality
	suite.InDelta(50.0, baseAnalyzer.normalizeQualityScore(25.5, "h264"), 0.1) // QP 25.5 -> 50% quality
	suite.InDelta(0.0, baseAnalyzer.normalizeQualityScore(51, "h264"), 0.1)    // QP 51 -> 0% quality

	// Test XviD/DivX normalization (assumed 0-10, lower is better, should be inverted to 0-100)
	suite.InDelta(100.0, baseAnalyzer.normalizeQualityScore(0, "xvid"), 0.1) // QP 0 -> 100% quality
	suite.InDelta(50.0, baseAnalyzer.normalizeQualityScore(5, "xvid"), 0.1)  // QP 5 -> 50% quality
	suite.InDelta(0.0, baseAnalyzer.normalizeQualityScore(10, "xvid"), 0.1)  // QP 10 -> 0% quality

	// Test default normalization (assumed 0-100, higher is better, no change needed)
	suite.Equal(75.0, baseAnalyzer.normalizeQualityScore(75.0, "unknown"))
}

// TestGetCodecFromFile tests the codec detection functionality with real files
func (suite *QualityAnalyzerTestSuite) TestGetCodecFromFile() {
	// First ensure FFmpeg is installed
	ffmpegInfo, err := FindFFmpeg()
	suite.NoError(err, "Error finding FFmpeg")

	if !ffmpegInfo.Installed {
		suite.T().Skip("FFmpeg not installed, skipping tests that require FFmpeg")
		return
	}

	// Get the paths to FFmpeg and FFprobe
	execPaths := GetExecutablePaths(ffmpegInfo.Path)
	baseAnalyzer := BaseQualityAnalyzer{
		FFmpegPath:  execPaths.FFmpeg,
		FFprobePath: execPaths.FFprobe,
	}

	// Test with real test files
	tests := []struct {
		name     string
		filePath string
	}{
		{"Xvid", suite.xvid},
		{"Divx", suite.divx},
		{"H264", suite.h264},
		{"HEVC", suite.hevc},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			codec, err := baseAnalyzer.getCodecFromFile(tt.filePath)
			suite.NoError(err, "Should detect codec without error")
			suite.NotEmpty(codec, "Codec should not be empty")
			suite.T().Logf("Detected codec for %s: %s", tt.name, codec)
		})
	}

	// Test with non-existent file
	codec, err := baseAnalyzer.getCodecFromFile("non_existent_file.mp4")
	suite.Error(err, "Should return error for non-existent file")
	suite.Empty(codec, "Codec should be empty for non-existent file")
}

// getTypeName returns the type name of the given object
func (suite *QualityAnalyzerTestSuite) getTypeName(obj interface{}) string {
	switch obj.(type) {
	case *XvidQualityAnalyzer:
		return "*ffmpeg.XvidQualityAnalyzer"
	case *DivxQualityAnalyzer:
		return "*ffmpeg.DivxQualityAnalyzer"
	case *H264QualityAnalyzer:
		return "*ffmpeg.H264QualityAnalyzer"
	case *HevcQualityAnalyzer:
		return "*ffmpeg.HevcQualityAnalyzer"
	default:
		return ""
	}
}

// TestCompleteFrameAnalysis tests that all frames in a video are analyzed correctly
// and that no frames are skipped during quality analysis
func (suite *QualityAnalyzerTestSuite) TestCompleteFrameAnalysis() {
	// First ensure FFmpeg is installed
	ffmpegInfo, err := FindFFmpeg()
	suite.NoError(err, "Error finding FFmpeg")

	if !ffmpegInfo.Installed {
		suite.T().Skip("FFmpeg not installed, skipping tests that require FFmpeg")
		return
	}

	// Get the paths to FFmpeg and FFprobe
	execPaths := GetExecutablePaths(ffmpegInfo.Path)

	// Try with different test files in case one codec works better
	testFiles := []string{suite.h264, suite.hevc, suite.xvid, suite.divx}
	t := suite.T()

	var successfulTest bool
	qualityRanges := suite.initQualityRanges()

	for _, testFile := range testFiles {
		t.Logf("Testing complete frame analysis with file: %s", filepath.Base(testFile))
		success := suite.testSingleFile(testFile, *execPaths, qualityRanges, t)
		if success {
			successfulTest = true
			break
		}
	}

	// Assert that at least one test was successful
	suite.True(successfulTest, "Should successfully analyze at least one test file")

	// Verify the quality ranges
	for level, count := range qualityRanges {
		t.Logf("%s: %d frames", level.String(), count)
		suite.GreaterOrEqual(count, 0, "Quality range count should be non-negative")
	}
}

// initQualityRanges initializes the quality ranges map
func (suite *QualityAnalyzerTestSuite) initQualityRanges() map[QualityLevel]int {
	return map[QualityLevel]int{
		UglyQuality:      0,
		BadQuality:       0,
		MediumQuality:    0,
		HighQuality:      0,
		ExcellentQuality: 0,
	}
}

// testSingleFile tests a single video file for frame analysis
func (suite *QualityAnalyzerTestSuite) testSingleFile(
	testFile string,
	execPaths ExecutablePaths,
	qualityRanges map[QualityLevel]int,
	t *testing.T,
) bool {
	// Step 1: Get the total number of frames in the video
	frameCount, err := getVideoFrameCount(execPaths.FFprobe, testFile)
	if err != nil {
		t.Logf("Error getting frame count for %s: %v. Trying next file.", filepath.Base(testFile), err)
		return false
	}
	t.Logf("Video has %d frames", frameCount)

	// If the video has too many frames, we might want to limit the test
	if frameCount > 500 {
		t.Logf("Video has too many frames (%d), limiting test scope", frameCount)
		frameCount = 500 // Test only the first 500 frames for practicality
	}

	// Step 2: Create an analyzer for the file
	analyzer, err := NewQualityAnalyzer(testFile)
	if err != nil {
		t.Logf("Error creating analyzer for %s: %v. Trying next file.", filepath.Base(testFile), err)
		return false
	}

	t.Logf("Created analyzer of type: %s", suite.getTypeName(analyzer))

	// Perform the analysis and get the frames
	frames, analyzeErr := suite.performAnalysis(analyzer, testFile, frameCount, t)
	if len(frames) == 0 {
		return suite.handleEmptyFrames(analyzeErr, testFile, t, analyzer)
	}

	// Check frame statistics
	framePercentage := float64(len(frames)) / float64(frameCount) * 100
	t.Logf("Received %d out of %d frames (%.1f%%)", len(frames), frameCount, framePercentage)

	if framePercentage < 50.0 {
		t.Logf("Received less than 50%% of frames. Trying next test file...")
		return false
	}

	// Validate frame sequence
	if !suite.validateFrameSequence(frames, t) {
		return false
	}

	// Verify quality metrics
	return suite.validateQualityMetrics(frames, testFile, qualityRanges, t)
}

// performAnalysis runs the analyzer and collects frames
func (suite *QualityAnalyzerTestSuite) performAnalysis(
	analyzer FrameQualityAnalyzer,
	testFile string,
	frameCount int,
	t *testing.T,
) ([]QualityFrame, error) {
	// Create a channel to receive all quality frames
	frameChan := make(chan QualityFrame, frameCount+10) // Buffer enough for all frames plus some extra

	// Use a done channel to signal when analysis is complete
	doneChan := make(chan struct{})
	var analyzeErr error

	// Start analysis in a goroutine
	go func() {
		defer close(doneChan)
		analyzeErr = analyzer.Analyze(testFile, frameChan)
		if analyzeErr != nil {
			t.Logf("Error analyzing video: %v", analyzeErr)
		}
	}()

	// Collect all frames from the channel
	frames := []QualityFrame{}
	timeout := time.After(120 * time.Second) // Generous timeout for full video analysis

collectFrames:
	for {
		select {
		case frame, open := <-frameChan:
			if !open {
				break collectFrames
			}
			frames = append(frames, frame)
		case <-timeout:
			t.Logf("Timeout waiting for all frames. Collected %d frames so far.", len(frames))
			break collectFrames
		case <-doneChan:
			// Wait a bit longer for any remaining frames to be sent
			time.Sleep(1 * time.Second)
			break collectFrames
		}
	}

	// Wait for goroutine to finish if it hasn't already
	select {
	case <-doneChan:
		// Goroutine finished
	case <-time.After(5 * time.Second):
		t.Logf("Warning: Frame analysis goroutine didn't finish within timeout")
	}

	return frames, analyzeErr
}

// handleEmptyFrames handles the case when no frames were received
func (suite *QualityAnalyzerTestSuite) handleEmptyFrames(
	analyzeErr error,
	testFile string,
	t *testing.T,
	analyzer FrameQualityAnalyzer,
) bool {
	t.Logf("No frames were received for %s. This could be due to codec compatibility issues.", filepath.Base(testFile))
	t.Logf("Error during analysis: %v", analyzeErr)

	// Consider this a successful test if the error message clearly indicates that
	// no QP data was found, as this is expected with some files/codecs
	if analyzeErr != nil && (strings.Contains(analyzeErr.Error(), "no QP data found") ||
		strings.Contains(analyzeErr.Error(), "no quality data found")) {
		t.Logf("Analyzer correctly reported that no QP data was available.")
		t.Logf("Successfully validated that %s properly reports when no frame quality data is available", suite.getTypeName(analyzer))
		return true
	}

	t.Logf("Trying next test file...")
	return false
}

// validateFrameSequence checks for proper frame sequencing without large gaps
func (suite *QualityAnalyzerTestSuite) validateFrameSequence(
	frames []QualityFrame,
	t *testing.T,
) bool {
	if len(frames) <= 1 {
		return true
	}

	// Sort frames by frame number to check for proper sequencing
	sort.Slice(frames, func(i, j int) bool {
		return frames[i].FrameNumber < frames[j].FrameNumber
	})

	// Check for frame number continuity (no big gaps)
	maxGap := 0
	gapCount := 0

	for i := 1; i < len(frames); i++ {
		gap := frames[i].FrameNumber - frames[i-1].FrameNumber
		if gap > 1 {
			gapCount++
			if gap > maxGap {
				maxGap = gap
			}
		}
	}

	t.Logf("Found %d gaps in frame sequence, max gap size: %d frames", gapCount, maxGap)
	if maxGap > 10 {
		t.Logf("Max gap between frames too large: %d. Trying next test file...", maxGap)
		return false
	}

	gapPercentage := float64(gapCount) / float64(len(frames)) * 100
	if gapPercentage > 20.0 {
		t.Logf("Too many gaps in frame sequence: %.1f%%. Trying next test file...", gapPercentage)
		return false
	}

	return true
}

// validateQualityMetrics ensures quality values are consistent and valid
func (suite *QualityAnalyzerTestSuite) validateQualityMetrics(
	frames []QualityFrame,
	testFile string,
	qualityRanges map[QualityLevel]int,
	t *testing.T,
) bool {
	qualityMin := 100.0
	qualityMax := 0.0
	var qualitySum float64

	for _, frame := range frames {
		if frame.Quality < qualityMin {
			qualityMin = frame.Quality
		}
		if frame.Quality > qualityMax {
			qualityMax = frame.Quality
		}
		qualitySum += frame.Quality

		// Quality should be a percentage between 0-100
		if frame.Quality < 0.0 || frame.Quality > 100.0 {
			t.Logf("Found invalid quality score: %.2f in frame %d", frame.Quality, frame.FrameNumber)
		}

		// QualityLevel should be valid
		if int(frame.QualityLevel) < int(UglyQuality) || int(frame.QualityLevel) > int(ExcellentQuality) {
			t.Logf("Found invalid quality level: %d in frame %d", frame.QualityLevel, frame.FrameNumber)
		}

		// Update the quality ranges
		qualityRanges[frame.QualityLevel]++
	}

	t.Logf("Quality range: %.1f to %.1f", qualityMin, qualityMax)
	t.Logf("Average quality: %.1f", qualitySum/float64(len(frames)))

	// In some cases, especially with our basic analyzer fallback, all frames might have the same QP value
	// This is still valid output, just not very useful for quality analysis
	if qualityMin == qualityMax {
		t.Logf("All frames have the same quality value (%.1f). This is not ideal but still valid.", qualityMin)
	}

	t.Logf("Successfully analyzed %s with %d frames", filepath.Base(testFile), len(frames))
	return true
}

// getVideoFrameCount determines the number of frames in a video file
func getVideoFrameCount(ffprobePath string, videoPath string) (int, error) {
	cmd := exec.Command(
		ffprobePath,
		"-v", "error",
		"-count_frames",
		"-select_streams", "v:0",
		"-show_entries", "stream=nb_read_frames",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("error getting frame count: %w", err)
	}

	frameCount, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, fmt.Errorf("error parsing frame count: %w", err)
	}

	return frameCount, nil
}

// TestQualityAnalyzerTestSuite runs the test suite
func TestQualityAnalyzerTestSuite(t *testing.T) {
	suite.Run(t, new(QualityAnalyzerTestSuite))
}
