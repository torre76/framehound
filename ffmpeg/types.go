// Package ffmpeg provides functionality for detecting and working with FFmpeg.
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

// AudioStream contains information about an audio stream in a container.
// AudioStream represents metadata and properties of an audio stream within a media container.
// It contains details such as format, codec, bitrate, and other audio-specific properties
// extracted from the media file. This type is used when reporting container information
// and for analyzing audio streams in the media file.
type AudioStream struct {
	// ID is the ID of the stream.
	ID string

	// Format is the format of the audio stream.
	Format string

	// FormatInfo is additional information about the format.
	FormatInfo string

	// CommercialName is the commercial name of the format.
	CommercialName string

	// CodecID is the ID of the codec.
	CodecID string

	// Duration is the duration of the audio stream in seconds.
	Duration float64

	// BitRateMode is the mode of the bit rate.
	BitRateMode string

	// BitRate is the bit rate of the audio stream in bits per second.
	BitRate int64

	// Channels is the number of channels in the audio stream.
	Channels int

	// ChannelLayout is the layout of the channels.
	ChannelLayout string

	// SamplingRate is the sampling rate of the audio stream in Hz.
	SamplingRate float64

	// FrameRate is the frame rate of the audio stream.
	FrameRate float64

	// CompressionMode is the mode of compression.
	CompressionMode string

	// StreamSize is the size of the audio stream in bytes.
	StreamSize int64

	// Title is the title of the audio stream.
	Title string

	// Language is the language of the audio stream.
	Language string

	// Default indicates whether this is the default audio stream.
	Default bool

	// Forced indicates whether this stream is forced.
	Forced bool
}

// BitrateAnalyzer provides methods to analyze bitrate values from video files.
// It uses FFprobe to extract frame-by-frame bitrate information from video files.
type BitrateAnalyzer struct {
	// FFprobePath is the path to the FFprobe executable
	FFprobePath string
	// mutex protects concurrent access to internal state
	mutex sync.Mutex
}


// ContainerInfo represents detailed metadata about a media container file.
// It provides a structured representation of the container's properties including
// general information and details about all contained streams (video, audio, and subtitle).
// This type is used for comprehensive media analysis and reporting container properties.
type ContainerInfo struct {
	// General contains general information about the container.
	General GeneralInfo

	// VideoStreams contains information about the video streams in the container.
	VideoStreams []VideoStream

	// AudioStreams contains information about the audio streams in the container.
	AudioStreams []AudioStream

	// SubtitleStreams contains information about the subtitle streams in the container.
	SubtitleStreams []SubtitleStream
}

// FFmpegInfo contains information about the FFmpeg installation
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

// FrameBitrateInfo represents the bitrate information for a single frame.
// It contains processed information about a video frame's bitrate and related metadata.
type FrameBitrateInfo struct {
	// FrameNumber is the frame number
	FrameNumber int `json:"frame_number"`
	// FrameType is the frame type (I, P, B)
	FrameType string `json:"frame_type"`
	// Bitrate is the bitrate of the frame in bits
	Bitrate int64 `json:"bitrate"`
	// PTS is the presentation timestamp of the frame
	PTS int64 `json:"pts"`
	// DTS is the decoding timestamp of the frame
	DTS int64 `json:"dts"`
}

// FrameQP represents QP (Quantization Parameter) information for a video frame.
// QP values determine the quality of the encoded video - lower values mean higher quality
// and higher bitrate, while higher values mean lower quality and lower bitrate.
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

// GeneralInfo contains general information about a media container.
type GeneralInfo struct {
	// UniqueID is the unique identifier of the container.
	UniqueID string

	// CompleteName is the complete name of the file.
	CompleteName string

	// Format is the container format.
	Format string

	// FormatVersion is the version of the container format.
	FormatVersion string

	// FileSize is the size of the file in bytes.
	FileSize int64

	// Duration is the duration of the media in seconds.
	Duration float64

	// OverallBitRate is the overall bit rate of the container in bits per second.
	OverallBitRate int64

	// FrameRate is the frame rate of the container.
	FrameRate float64

	// EncodedDate is the date when the file was encoded.
	EncodedDate string

	// WritingApplication is the application used to write the file.
	WritingApplication string

	// WritingLibrary is the library used to write the file.
	WritingLibrary string
}

// Prober provides methods to probe video files for information
type Prober struct {
	// FFprobePath is the path to the FFprobe executable
	FFprobePath string
}

// QPAnalyzer analyzes the QP (Quantization Parameter) values of video frames.
// QP values indicate the compression level applied to different parts of the video,
// with lower values representing higher quality and higher values representing lower quality.
type QPAnalyzer struct {
	// FFmpegPath is the path to the FFmpeg executable
	FFmpegPath string

	// SupportsQPReading indicates whether the installed FFmpeg supports QP reading
	SupportsQPReading bool

	// prober is used to check video codec compatibility
	prober *Prober

	// mutex protects concurrent access to the analyzer
	mutex sync.Mutex
}

// SubtitleStream contains information about a subtitle stream in a container.
type SubtitleStream struct {
	// ID is the ID of the stream.
	ID string

	// Format is the format of the subtitle stream.
	Format string

	// CodecID is the ID of the codec.
	CodecID string

	// CodecIDInfo is additional information about the codec ID.
	CodecIDInfo string

	// Duration is the duration of the subtitle stream in seconds.
	Duration float64

	// BitRate is the bit rate of the subtitle stream in bits per second.
	BitRate int64

	// FrameRate is the frame rate of the subtitle stream.
	FrameRate float64

	// CountOfElements is the number of elements in the subtitle stream.
	CountOfElements int

	// StreamSize is the size of the subtitle stream in bytes.
	StreamSize int64

	// Title is the title of the subtitle stream.
	Title string

	// Language is the language of the subtitle stream.
	Language string

	// Default indicates whether this is the default subtitle stream.
	Default bool

	// Forced indicates whether this stream is forced.
	Forced bool
}

// VideoInfo contains information about a video file
type VideoInfo struct {
	// Codec is the video codec name (h264, hevc, etc.)
	Codec string
	// Width is the video width in pixels
	Width int
	// Height is the video height in pixels
	Height int
	// FrameRate is the video frame rate in frames per second
	FrameRate float64
	// Duration is the video duration in seconds
	Duration float64
	// FilePath is the path to the video file
	FilePath string
}

// VideoStream contains information about a video stream in a container.
type VideoStream struct {
	// ID is the ID of the stream.
	ID string

	// Format is the format of the video stream.
	Format string

	// FormatInfo is additional information about the format.
	FormatInfo string

	// FormatProfile is the profile of the format.
	FormatProfile string

	// FormatSettings is the settings of the format.
	FormatSettings string

	// CodecID is the ID of the codec.
	CodecID string

	// Duration is the duration of the video stream in seconds.
	Duration float64

	// BitRate is the bit rate of the video stream in bits per second.
	BitRate int64

	// Width is the width of the video in pixels.
	Width int

	// Height is the height of the video in pixels.
	Height int

	// DisplayAspectRatio is the display aspect ratio of the video.
	DisplayAspectRatio float64

	// FrameRateMode is the mode of the frame rate.
	FrameRateMode string

	// FrameRate is the frame rate of the video.
	FrameRate float64

	// ColorSpace is the color space of the video.
	ColorSpace string

	// ChromaSubsampling is the chroma subsampling of the video.
	ChromaSubsampling string

	// BitDepth is the bit depth of the video.
	BitDepth int

	// ScanType is the scan type of the video.
	ScanType string

	// StreamSize is the size of the video stream in bytes.
	StreamSize int64

	// Title is the title of the video stream.
	Title string

	// Language is the language of the video stream.
	Language string

	// Default indicates whether this is the default video stream.
	Default bool

	// Forced indicates whether this stream is forced.
	Forced bool
}
