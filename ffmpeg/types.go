// Package ffmpeg provides functionality for detecting and working with FFmpeg.
// It offers capabilities for analyzing video files, extracting metadata, and
// processing frame-level information such as bitrates and quality parameters.
package ffmpeg

import (
	"encoding/json"
	"sync"
)

// Private types (alphabetical)

// ffprobeFrameInfo represents the JSON structure returned by FFprobe for a single frame.
// It contains detailed information about a video frame extracted from FFprobe's JSON output.
type ffprobeFrameInfo struct {
	MediaType      string      `json:"media_type"`
	StreamIndex    int         `json:"stream_index"`
	KeyFrame       int         `json:"key_frame"`
	PktPts         json.Number `json:"pkt_pts"`
	PktPtsTime     string      `json:"pkt_pts_time"`
	PktDts         json.Number `json:"pkt_dts"`
	PktDtsTime     string      `json:"pkt_dts_time"`
	BestEffortPts  json.Number `json:"best_effort_pts"`
	PktDuration    json.Number `json:"pkt_duration"`
	PktSize        json.Number `json:"pkt_size"`
	Width          int         `json:"width"`
	Height         int         `json:"height"`
	PictType       string      `json:"pict_type"`
	CodedPictNum   int         `json:"coded_picture_number"`
	DisplayPictNum int         `json:"display_picture_number"`
}

// Public types (alphabetical)

// AudioStream encapsulates information about an audio stream found in a media file.
// It provides access to key audio properties such as codec, channels, bitrate and duration.
type AudioStream struct {
	Index         int     // Stream index
	Format        string  // Audio codec name
	FormatFull    string  // Full codec name
	Channels      int     // Number of audio channels
	ChannelLayout string  // Layout of audio channels
	SamplingRate  int     // Audio sampling rate in Hz
	BitRate       int64   // Bit rate in bits per second
	Duration      float64 // Duration in seconds
	Language      string  // Language code
	Title         string  // Stream title
}

// AttachmentStream represents an attachment embedded within a media container.
// Attachments typically include fonts, thumbnails, or other auxiliary files.
type AttachmentStream struct {
	Index    int    // Stream index
	FileName string // Attached file name
	MimeType string // MIME type
}

// BitrateAnalyzer provides methods to analyze frame-by-frame bitrate information from video files.
// It extracts detailed bitrate statistics using FFprobe and offers ways to process this data.
type BitrateAnalyzer struct {
	// FFprobePath is the path to the FFprobe executable
	FFprobePath string
	// mutex protects concurrent access to internal state
	mutex sync.Mutex
}

// ChapterStream represents a chapter marker within a media file.
// Chapters allow navigation to specific points in the media content.
type ChapterStream struct {
	ID        int64   // Chapter ID
	StartTime float64 // Start time in seconds
	EndTime   float64 // End time in seconds
	Title     string  // Chapter title
}

// ContainerInfo contains comprehensive information about a media container file.
// It aggregates details about all streams and general container metadata.
type ContainerInfo struct {
	General           GeneralInfo        // General container information
	VideoStreams      []VideoStream      // Video streams
	AudioStreams      []AudioStream      // Audio streams
	SubtitleStreams   []SubtitleStream   // Subtitle streams
	ChapterStreams    []ChapterStream    // Chapter streams
	AttachmentStreams []AttachmentStream // Attachment streams
	DataStreams       []DataStream       // Data streams
	OtherStreams      []OtherStream      // Other streams
}

// CUAnalyzer handles the analysis of coding units (CU) in HEVC encoded videos.
// It extracts information about the size and depth of coding units at frame level
// using FFmpeg with special debug flags enabled.
type CUAnalyzer struct {
	// FFmpegPath is the path to the FFmpeg executable
	FFmpegPath string

	// SupportsCUReading indicates whether the installed FFmpeg supports CU reading for HEVC
	SupportsCUReading bool

	// prober is used to check video codec compatibility
	prober *Prober
}

// DataStream represents a data stream contained within a media file.
// Data streams typically contain information not meant for direct playback.
type DataStream struct {
	Index      int    // Stream index
	Format     string // Data codec name
	FormatFull string // Full codec name
	Title      string // Stream title
}

// FFmpegInfo stores information about the FFmpeg installation on the system.
// It tracks capabilities, location, and version of the FFmpeg tools.
type FFmpegInfo struct {
	// Installed is true if FFmpeg is found in the system
	Installed bool
	// Path is the full path to the FFmpeg executable
	Path string
	// Version is the version of FFmpeg
	Version string
	// HasQPReadingInfoSupport is true if FFmpeg supports QP value reading
	HasQPReadingInfoSupport bool
	// HasCUReadingInfoSupport is true if FFmpeg supports CU value reading
	HasCUReadingInfoSupport bool
}

// FrameBitrateInfo captures bitrate information for a single video frame.
// It provides detailed statistics about frame size, type, and timestamps.
type FrameBitrateInfo struct {
	// FrameNumber is the sequential number of the frame
	FrameNumber int `json:"frame_number"`
	// FrameType indicates the frame type (I, P, B)
	FrameType string `json:"frame_type"`
	// Bitrate represents the size of the frame in bits
	Bitrate int64 `json:"bitrate"`
	// PTS is the presentation timestamp of the frame
	PTS int64 `json:"pts"`
	// DTS is the decoding timestamp of the frame
	DTS int64 `json:"dts"`
}

// FrameCU contains CU (Coding Unit) information for a single HEVC video frame.
// It tracks how the frame is subdivided for encoding, which reflects the complexity
// of different regions within the frame.
type FrameCU struct {
	// FrameNumber is the sequential number of the frame in the output
	FrameNumber int

	// OriginalFrameNumber is the actual frame number in the source
	OriginalFrameNumber int

	// FrameType is the type of frame (I, P, B)
	FrameType string

	// CodecType is the video codec (hevc, h265)
	CodecType string

	// CUSizes contains all the CU sizes (width*height) for this frame
	CUSizes []int

	// AverageCUSize is the average of all CU sizes for this frame
	AverageCUSize float64
}

// FrameQP holds QP (Quantization Parameter) information for a single video frame.
// It tracks the compression level applied to the frame, with lower QP values
// indicating higher quality and higher values indicating more aggressive compression.
type FrameQP struct {
	// FrameNumber is the sequential number of the frame in the output
	FrameNumber int

	// OriginalFrameNumber is the actual frame number in the source
	OriginalFrameNumber int

	// FrameType is the type of frame (I, P, B)
	FrameType string

	// CodecType is the video codec (h264, mpeg4, etc.)
	CodecType string

	// QPValues contains all the QP values for this frame
	QPValues []int

	// AverageQP is the average of all QP values for this frame
	AverageQP float64
}

// GeneralInfo provides general metadata about a media container.
// It includes details like format, duration, and file size that apply to the container as a whole.
type GeneralInfo struct {
	Format      string            // Container format
	BitRate     string            // Overall bit rate
	Duration    string            // Duration as a string
	DurationF   float64           // Duration in seconds as a float
	Size        string            // File size
	StartTime   string            // Start time
	StreamCount int               // Number of streams
	Tags        map[string]string // Metadata tags
}

// OtherStream represents any stream type in a media file that doesn't fit into standard categories.
// It provides a way to access information about specialized or uncommon stream types.
type OtherStream struct {
	Index      int    // Stream index
	Type       string // Stream type
	Format     string // Stream codec name
	FormatFull string // Full codec name
}

// Prober facilitates the extraction of media file information using FFprobe.
// It provides methods to analyze container formats and stream properties.
type Prober struct {
	FFmpegInfo *FFmpegInfo // Information about the FFmpeg installation
}

// QPAnalyzer handles the extraction and analysis of Quantization Parameter (QP) values
// from video frames. It processes the debug output from FFmpeg to gather QP data
// that is essential for quality and compression analysis.
type QPAnalyzer struct {
	// FFmpegPath is the path to the FFmpeg executable
	FFmpegPath string

	// SupportsQPReading indicates whether the installed FFmpeg supports QP reading
	SupportsQPReading bool

	// prober is used to check video codec compatibility
	prober *Prober
}

// SubtitleStream contains information about a subtitle stream in a media file.
// It provides access to properties like format, language, and title.
type SubtitleStream struct {
	Index      int    // Stream index
	Format     string // Subtitle codec name
	FormatFull string // Full codec name
	Language   string // Language code
	Title      string // Stream title
}

// VideoStream encapsulates detailed information about a video stream in a media file.
// It exposes comprehensive properties including format, dimensions, frame rate, and more.
type VideoStream struct {
	Index              int     // Stream index
	Format             string  // Video codec name
	FormatFull         string  // Full codec name
	FormatProfile      string  // Codec profile
	Width              int     // Frame width in pixels
	Height             int     // Frame height in pixels
	DisplayAspectRatio float64 // Display aspect ratio
	PixelAspectRatio   float64 // Pixel aspect ratio
	FrameRate          float64 // Frames per second
	FrameRateMode      string  // Frame rate mode (CFR, VFR)
	BitRate            int64   // Bit rate in bits per second
	BitDepth           int     // Bit depth
	Duration           float64 // Duration in seconds
	ColorSpace         string  // Color space
	ScanType           string  // Scan type (progressive, interlaced)
	HasBFrames         bool    // Whether the stream has B-frames
	Language           string  // Language code
	Title              string  // Stream title
}
