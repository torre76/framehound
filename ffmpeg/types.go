// Package ffmpeg provides functionality for detecting and working with FFmpeg.
// It offers capabilities for analyzing video files, extracting metadata, and
// processing frame-level information such as bitrates, quality parameters, and
// quality metrics including QP values, PSNR, SSIM, and VMAF.
package ffmpeg

import (
	"encoding/json"
	"regexp"
	"sync"
)

// Private types (alphabetical)

// chapterOutput represents a chapter's metadata in the ffprobe JSON output.
type chapterOutput struct {
	ID        int64             `json:"id"`
	TimeBase  string            `json:"time_base"`
	Start     int64             `json:"start"`
	StartTime string            `json:"start_time"`
	End       int64             `json:"end"`
	EndTime   string            `json:"end_time"`
	Tags      map[string]string `json:"tags,omitempty"`
}

// ffprobeFormatOutput represents a container's format metadata in the ffprobe JSON output.
type ffprobeFormatOutput struct {
	Filename         string            `json:"filename"`
	NBStreams        int               `json:"nb_streams"`
	NBPrograms       int               `json:"nb_programs"`
	FormatName       string            `json:"format_name"`
	FormatLongName   string            `json:"format_long_name"`
	StartTime        string            `json:"start_time"`
	Duration         string            `json:"duration"`
	Size             string            `json:"size"`
	BitRate          string            `json:"bit_rate"`
	ProbeScore       int               `json:"probe_score"`
	Tags             map[string]string `json:"tags,omitempty"`
	Chapters         []chapterOutput   `json:"chapters,omitempty"`
	FormatProperties map[string]string `json:"format_properties,omitempty"`
}

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

// ffprobeOutput represents the complete output from ffprobe.
type ffprobeOutput struct {
	Streams  []ffprobeStreamOutput `json:"streams"`
	Format   ffprobeFormatOutput   `json:"format"`
	Chapters []chapterOutput       `json:"chapters,omitempty"`
}

// ffprobeStreamOutput represents a stream's metadata in the ffprobe JSON output.
type ffprobeStreamOutput struct {
	Index              int               `json:"index"`
	CodecName          string            `json:"codec_name"`
	CodecLongName      string            `json:"codec_long_name"`
	Profile            string            `json:"profile"`
	CodecType          string            `json:"codec_type"`
	CodecTagString     string            `json:"codec_tag_string"`
	CodecTag           string            `json:"codec_tag"`
	Width              int               `json:"width,omitempty"`
	Height             int               `json:"height,omitempty"`
	SampleRate         string            `json:"sample_rate,omitempty"`
	Channels           int               `json:"channels,omitempty"`
	ChannelLayout      string            `json:"channel_layout,omitempty"`
	BitsPerSample      int               `json:"bits_per_sample,omitempty"`
	HasBFrames         int               `json:"has_b_frames,omitempty"`
	SampleAspectRatio  string            `json:"sample_aspect_ratio,omitempty"`
	DisplayAspectRatio string            `json:"display_aspect_ratio,omitempty"`
	BitRate            string            `json:"bit_rate,omitempty"`
	BitsPerRawSample   string            `json:"bits_per_raw_sample,omitempty"`
	FrameRate          string            `json:"r_frame_rate,omitempty"`
	ColorRange         string            `json:"color_range,omitempty"`
	ColorSpace         string            `json:"color_space,omitempty"`
	PixFmt             string            `json:"pix_fmt,omitempty"`
	FieldOrder         string            `json:"field_order,omitempty"`
	TimeBase           string            `json:"time_base,omitempty"`
	Duration           string            `json:"duration,omitempty"`
	DurationTs         int64             `json:"duration_ts,omitempty"`
	StartPts           int64             `json:"start_pts,omitempty"`
	StartTime          string            `json:"start_time,omitempty"`
	DispositionObj     map[string]int    `json:"disposition,omitempty"`
	Tags               map[string]string `json:"tags,omitempty"`
}

// StreamInfo holds common information for different stream types.
type StreamInfo struct {
	Index      int
	Format     string
	FormatFull string
	Title      string
	Language   string
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

// DataStream represents a data stream contained within a media file.
// Data streams typically contain information not meant for direct playback.
type DataStream struct {
	Index      int    // Stream index
	Format     string // Data codec name
	FormatFull string // Full codec name
	Title      string // Stream title
}

// ExecutablePaths contains the paths to FFmpeg and FFprobe executables.
// This structure is useful for applications that need both executables.
type ExecutablePaths struct {
	// FFmpeg is the path to the FFmpeg executable
	FFmpeg string

	// FFprobe is the path to the FFprobe executable
	FFprobe string
}

// FFmpegInfo contains information about the FFmpeg installation.
// It provides details about the path, version, and capabilities of the installed FFmpeg.
type FFmpegInfo struct {
	// Path is the absolute path to the FFmpeg executable
	Path string

	// Version is the FFmpeg version string
	Version string

	// Installed indicates whether FFmpeg is installed and available
	Installed bool

	// HasQPReadingInfoSupport indicates whether the FFmpeg installation supports QP reading
	HasQPReadingInfoSupport bool
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

// FrameQP represents the QP (Quantization Parameter) data for a single video frame.
// It contains information about the frame type, frame number, QP values, average QP, and codec type.
type FrameQP struct {
	// FrameNumber is the sequential number of the frame in the video
	FrameNumber int `json:"frame_number"`

	// FrameType is the type of the frame (I, P, B, or ?)
	FrameType string `json:"frame_type"`

	// QPValues contains all the QP values extracted for this frame
	QPValues []int `json:"qp_values,omitempty"`

	// AverageQP is the average QP value for this frame
	AverageQP float64 `json:"average_qp"`

	// CodecType indicates the codec of this frame (h264, hevc, etc.)
	CodecType string `json:"codec_type,omitempty"`
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

// Prober provides methods for probing media containers.
// It uses FFmpeg's capabilities to extract detailed information about media files.
type Prober struct {
	// FFmpegInfo contains the FFmpeg installation details used by this prober
	FFmpegInfo *FFmpegInfo
}

// ProberInterface defines the interface for container probing functionality.
// It allows for dependency injection and testing by abstracting the probing implementation.
type ProberInterface interface {
	// GetExtendedContainerInfo retrieves detailed information about a media container file
	GetExtendedContainerInfo(filePath string) (*ContainerInfo, error)
}

// PSNRMetrics contains Peak Signal-to-Noise Ratio measurements.
// PSNR is a traditional metric for measuring video quality.
type PSNRMetrics struct {
	// Y represents PSNR for the Y (luma) channel
	Y float64 `json:"y"`

	// U represents PSNR for the U (chroma) channel
	U float64 `json:"u"`

	// V represents PSNR for the V (chroma) channel
	V float64 `json:"v"`

	// Average is the average PSNR across all channels
	Average float64 `json:"average"`
}

// QPAnalyzer extracts QP (Quantization Parameter) data from video files.
// It uses FFmpeg's debug mode to extract QP values for H.264 and HEVC videos.
type QPAnalyzer struct {
	// FFmpegPath is the path to the FFmpeg executable
	FFmpegPath string

	// SupportsQPAnalysis indicates whether the installed FFmpeg supports QP analysis
	SupportsQPAnalysis bool

	// prober is used for getting video container information
	prober ProberInterface

	// Generic regex patterns (fallback)
	genericNewFrameRegex  *regexp.Regexp
	genericFrameTypeRegex *regexp.Regexp
	genericQPRegex        *regexp.Regexp

	// H.264 specific regex patterns
	h264NewFrameRegex  *regexp.Regexp
	h264FrameTypeRegex *regexp.Regexp
	h264QPRegex        *regexp.Regexp

	// HEVC specific regex patterns
	hevcNewFrameRegex  *regexp.Regexp
	hevcFrameTypeRegex *regexp.Regexp
	hevcQPRegex        *regexp.Regexp
}

// QPReport contains comprehensive QP analysis statistics for a video file.
// It includes overall and per-frame-type QP statistics.
type QPReport struct {
	// Filename is the name of the analyzed video file
	Filename string `json:"filename"`

	// TotalFrames is the total number of frames analyzed
	TotalFrames int `json:"total_frames"`

	// AverageQP is the average QP value across all frames
	AverageQP float64 `json:"average_qp"`

	// MinQP is the minimum average QP value found in any frame
	MinQP float64 `json:"min_qp"`

	// MaxQP is the maximum average QP value found in any frame
	MaxQP float64 `json:"max_qp"`

	// CodecType indicates the codec of the video (h264, hevc, etc.)
	CodecType string `json:"codec_type,omitempty"`

	// Percentiles contains key percentiles of the QP distribution (P10, P50, P90, etc.)
	Percentiles map[string]float64 `json:"percentiles,omitempty"`

	// TotalQP is the sum of all frame average QP values (used for calculations)
	TotalQP float64 `json:"-"`

	// QPValues contains all individual QP values from all frames
	QPValues []int `json:"-"`

	// FrameData stores frames grouped by frame type
	FrameData map[string][]FrameQP `json:"frame_data"`

	// AverageQPByType stores the average QP value for each frame type
	AverageQPByType map[string]float64 `json:"average_qp_by_type"`
}

// QPReportSummary provides a condensed view of QP analysis statistics.
// It contains key statistical data about QP values in the video.
type QPReportSummary struct {
	// TotalFrames is the total number of frames analyzed
	TotalFrames int `json:"total_frames"`

	// AverageQP is the average QP value across all frames
	AverageQP float64 `json:"average_qp"`

	// MinQP is the minimum average QP value found in any frame
	MinQP float64 `json:"min_qp"`

	// MaxQP is the maximum average QP value found in any frame
	MaxQP float64 `json:"max_qp"`

	// CodecType indicates the codec of the video
	CodecType string `json:"codec_type,omitempty"`

	// Percentiles contains key percentiles (P10, P50, P90, etc.)
	Percentiles map[string]float64 `json:"percentiles,omitempty"`

	// AverageQPByType stores the average QP value for each frame type
	AverageQPByType map[string]float64 `json:"average_qp_by_type"`
}

// QualityAnalyzer provides functionality for analyzing video quality metrics.
// It supports various codecs and can extract quality information from video files.
type QualityAnalyzer struct {
	// FFmpegPath is the path to the FFmpeg executable
	FFmpegPath string

	// SupportedCodecs contains the list of codecs supported by this analyzer
	SupportedCodecs []string

	// prober is used to check video codec compatibility
	prober ProberInterface

	// Quality analyzers that may be used by this analyzer
	qpAnalyzer *QPAnalyzer
}

// QualityMetrics contains video quality comparison metrics.
// It includes PSNR, SSIM, and VMAF measurements when available.
type QualityMetrics struct {
	// PSNR values if calculated
	PSNR *PSNRMetrics `json:"psnr,omitempty"`

	// SSIM values if calculated
	SSIM *SSIMMetrics `json:"ssim,omitempty"`

	// VMAF values if calculated
	VMAF *VMAFMetrics `json:"vmaf,omitempty"`
}

// QualityReport contains quality analysis data for a video file.
// It includes video information, quality metrics, and QP analysis if available.
type QualityReport struct {
	// Filename is the name of the analyzed video file
	Filename string `json:"filename"`

	// VideoInfo contains general information about the video
	VideoInfo VideoInfo `json:"video_info"`

	// QualityMetrics contains the quality metrics for the video
	QualityMetrics QualityMetrics `json:"quality_metrics,omitempty"`

	// QPReportSummary contains a summary of QP analysis if available
	QPReportSummary *QPReportSummary `json:"qp_report_summary,omitempty"`
}

// SSIMMetrics contains Structural Similarity Index measurements.
// SSIM evaluates the perceived quality difference between two images.
type SSIMMetrics struct {
	// Y represents SSIM for the Y (luma) channel
	Y float64 `json:"y"`

	// U represents SSIM for the U (chroma) channel
	U float64 `json:"u"`

	// V represents SSIM for the V (chroma) channel
	V float64 `json:"v"`

	// Average is the average SSIM across all channels
	Average float64 `json:"average"`
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

// VideoInfo contains basic information about a video file.
// It provides metadata such as codec, dimensions, and duration.
type VideoInfo struct {
	// Codec is the video codec name
	Codec string `json:"codec"`

	// Width is the width of the video in pixels
	Width int `json:"width"`

	// Height is the height of the video in pixels
	Height int `json:"height"`

	// FrameRate is the frame rate of the video
	FrameRate float64 `json:"frame_rate"`

	// BitRate is the bit rate of the video in bits per second
	BitRate int64 `json:"bit_rate"`

	// Duration is the duration of the video in seconds
	Duration float64 `json:"duration"`
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

// VMAFMetrics contains Video Multi-method Assessment Fusion measurements.
// VMAF is a perceptual video quality metric that combines multiple features.
type VMAFMetrics struct {
	// Score is the VMAF score (0-100)
	Score float64 `json:"score"`

	// Min is the minimum VMAF score
	Min float64 `json:"min"`

	// Max is the maximum VMAF score
	Max float64 `json:"max"`
}
