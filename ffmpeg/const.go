// Package ffmpeg provides functionality for detecting and working with FFmpeg.
// It includes tools for analyzing video files, extracting frame information,
// and processing bitrate and QP (Quantization Parameter) data.
package ffmpeg

// Private constants (alphabetical)
// Commented out unused constants:
// const (
//     // defaultTimeout is the standard timeout for FFmpeg operations.
//     defaultTimeout = 30 // seconds
//
//     // errorPrefix is used for formatting error messages.
//     errorPrefix = "ffmpeg: "
// )

// Public constants (alphabetical)
const (
	// DefaultFrameRate is the standard frame rate used when no frame rate is specified.
	DefaultFrameRate = 24.0

	// DefaultBitrate is the standard bitrate used when no bitrate is specified.
	DefaultBitrate = "1M"

	// MaxConcurrentOperations is the maximum number of concurrent FFmpeg operations.
	MaxConcurrentOperations = 4
)
