// Package ffmpeg provides functionality for detecting and working with FFmpeg.
// It includes tools for analyzing video files, extracting frame information,
// and processing bitrate and QP (Quantization Parameter) data.
package ffmpeg

import (
	"fmt"
	"time"
)

// Private constants (alphabetical)
const (
	// defaultTimeout is the standard timeout in seconds for FFmpeg operations.
	// Operations that exceed this timeout will be terminated.
	defaultTimeout = 30 * time.Second

	// errorPrefix is used as a prefix for all error messages from this package.
	// This ensures consistent error formatting across the package.
	errorPrefix = "ffmpeg: "
)

// Public constants (alphabetical)
const (
	// DefaultFrameRate defines the standard frame rate used when no frame rate is specified.
	// This value is commonly used in video production workflows.
	DefaultFrameRate = 24.0

	// DefaultBitrate specifies the standard bitrate used when no bitrate is specified.
	// The value "1M" represents 1 megabit per second.
	DefaultBitrate = "1M"

	// MaxConcurrentOperations defines the maximum number of concurrent FFmpeg operations
	// allowed to prevent system resource exhaustion.
	MaxConcurrentOperations = 4
)

// Public functions (alphabetical)

// GetDefaultTimeout returns the standard timeout duration for FFmpeg operations.
// Applications can use this when creating contexts or setting command timeouts.
func GetDefaultTimeout() time.Duration {
	return defaultTimeout
}

// FormatError creates a standardized error message with the package prefix.
// It ensures all errors from this package have a consistent format and can be
// easily identified as originating from the ffmpeg package.
func FormatError(format string, args ...interface{}) error {
	return fmt.Errorf(errorPrefix+format, args...)
}
