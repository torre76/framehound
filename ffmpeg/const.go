// Package ffmpeg provides functionality for detecting and working with FFmpeg.
// It offers capabilities for analyzing video files, extracting metadata, and
// processing frame-level information such as bitrates, quality parameters, and
// quality metrics including QP values, PSNR, SSIM, and VMAF.
package ffmpeg

import (
	"fmt"
	"time"
)

// Private constants (alphabetical)
const (
	// codecH264 is the identifier for H.264/AVC codec in FFmpeg.
	codecH264 = "h264"

	// codecHEVC is the identifier for H.265/HEVC codec in FFmpeg.
	codecHEVC = "hevc"

	// defaultTimeout is the standard timeout in seconds for FFmpeg operations.
	// Operations that exceed this timeout will be terminated.
	defaultTimeout = 30 * time.Second

	// errorPrefix is used as a prefix for all error messages from this package.
	// This ensures consistent error formatting across the package.
	errorPrefix = "ffmpeg: "

	// qualityCodecDivx is the identifier for DivX codec in quality analysis.
	qualityCodecDivx = "divx"

	// qualityCodecH264 is the identifier for H.264/AVC codec in quality analysis.
	qualityCodecH264 = "h264"

	// qualityCodecHEVC is the identifier for H.265/HEVC codec in quality analysis.
	qualityCodecHEVC = "hevc"

	// qualityCodecMPEG4 is the identifier for MPEG-4 Part 2 codec in quality analysis.
	qualityCodecMPEG4 = "mpeg4"

	// qualityCodecXvid is the identifier for Xvid codec in quality analysis.
	qualityCodecXvid = "xvid"

	// qpChannelBufferSize defines the buffer size for the channel used in QP analysis.
	// This helps prevent blocking when processing frame data.
	qpChannelBufferSize = 100

	// scannerBufferSize defines the buffer size for reading FFmpeg output during QP analysis.
	// 10MB buffer to handle large output lines in debug mode.
	scannerBufferSize = 10 * 1024 * 1024 // 10MB
)

// Public constants (alphabetical)
const (
	// DefaultBitrate specifies the standard bitrate used when no bitrate is specified.
	// The value "1M" represents 1 megabit per second.
	DefaultBitrate = "1M"

	// DefaultFrameRate defines the standard frame rate used when no frame rate is specified.
	// This value is commonly used in video production workflows.
	DefaultFrameRate = 24.0

	// MaxConcurrentOperations defines the maximum number of concurrent FFmpeg operations
	// allowed to prevent system resource exhaustion.
	MaxConcurrentOperations = 4
)

// Public functions (alphabetical)

// FormatError creates a standardized error message with the package prefix.
// It ensures all errors from this package have a consistent format and can be
// easily identified as originating from the ffmpeg package.
func FormatError(format string, args ...interface{}) error {
	return fmt.Errorf(errorPrefix+format, args...)
}

// GetDefaultTimeout returns the standard timeout duration for FFmpeg operations.
// Applications can use this when creating contexts or setting command timeouts.
func GetDefaultTimeout() time.Duration {
	return defaultTimeout
}
