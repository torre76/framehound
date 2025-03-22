// Package ffmpeg provides functionality for interacting with FFmpeg
// and extracting information from media files.
package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Private constants (alphabetical)

// titleFieldsRegex is a regular expression to match whitespace and special characters in titles.
var titleFieldsRegex = regexp.MustCompile(`[\s._-]+`)

// Public constants (alphabetical)
// None currently defined

// Private variables (alphabetical)
// None currently defined

// Public variables (alphabetical)
// None currently defined

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

// Private functions (alphabetical)

// cleanFilename transforms a filename into a clean, readable title by removing
// common video file artifacts like resolution, codec names, and release tags.
// It ensures consistent formatting for display purposes in user interfaces.
func cleanFilename(filename string) string {
	// Remove file extension
	base := filepath.Base(filename)
	name := strings.TrimSuffix(base, filepath.Ext(base))

	// Replace special characters with spaces
	name = titleFieldsRegex.ReplaceAllString(name, " ")

	// Remove common suffixes
	suffixes := []string{
		"1080p", "720p", "480p", "360p", "240p",
		"bdrip", "brrip", "bluray", "dvdrip", "webrip", "web-dl", "web",
		"hevc", "x264", "x265", "h264", "h265", "h 264", "h 265",
		"aac", "ac3", "dts", "hdtv", "pdtv", "proper", "internal",
		"xvid", "divx", "retail", "repack", "extended", "unrated",
		"multi", "multisubs", "dubbed", "subbed", "subs", "hardcoded",
	}

	lowerName := strings.ToLower(name)
	for _, suffix := range suffixes {
		pattern := " " + suffix + "$"
		if regexp.MustCompile(pattern).MatchString(lowerName) {
			name = regexp.MustCompile("(?i)"+pattern).ReplaceAllString(name, "")
		}
	}

	// Trim spaces
	name = strings.TrimSpace(name)

	// Format the title
	return formatAsTitle(name)
}

// formatAsTitle converts a string to title case following proper English title
// capitalization rules for articles, prepositions, and conjunctions.
// It handles special cases such as acronyms and words that should remain uppercase.
func formatAsTitle(s string) string {
	// Words to keep lowercase
	lowerWords := map[string]bool{
		"a": true, "an": true, "the": true,
		"and": true, "but": true, "or": true, "nor": true,
		"in": true, "on": true, "at": true, "by": true, "for": true, "with": true, "to": true, "from": true,
		"of": true,
	}

	// Words to keep uppercase
	upperWords := map[string]bool{
		"id": true, "tv": true, "ii": true, "iii": true, "iv": true, "v": true, "vi": true,
		"vii": true, "viii": true, "ix": true, "x": true, "xi": true, "xii": true,
		"uk": true, "usa": true, "us": true, "eu": true, "ufo": true, "un": true, "nato": true,
	}

	words := strings.Fields(s)
	for i, word := range words {
		// Skip empty words
		if word == "" {
			continue
		}

		// Check if word should be all uppercase
		wordLower := strings.ToLower(word)
		if upperWords[wordLower] {
			words[i] = strings.ToUpper(wordLower)
			continue
		}

		// For other words, capitalize first letter unless
		// it's a lowercase word not at the beginning or end
		if i > 0 && i < len(words)-1 && lowerWords[wordLower] {
			words[i] = wordLower
		} else {
			runes := []rune(wordLower)
			if len(runes) > 0 {
				runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
			}
			words[i] = string(runes)
		}
	}

	return strings.Join(words, " ")
}

// getContainerTitle extracts a user-friendly title from the container's metadata.
// It first checks for an explicit title tag, then falls back to the filename,
// and finally uses stream titles if no better option is available.
func getContainerTitle(info *ContainerInfo) string {
	// Try to get the filename
	if info.General.Tags != nil {
		// First check for standard title tag
		if title, ok := info.General.Tags["title"]; ok && title != "" {
			return title
		}

		// Then check for filename
		if filename, ok := info.General.Tags["file_path"]; ok && filename != "" {
			return cleanFilename(filename)
		}
	}

	// If no title or filename, use the first video stream title if available
	if len(info.VideoStreams) > 0 && info.VideoStreams[0].Title != "" {
		return info.VideoStreams[0].Title
	}

	// Last resort: return a generic title
	return "Untitled Media"
}

// removeUnicodeZeroWidthChars strips invisible Unicode characters from a string.
// These characters can cause display issues in terminals and text interfaces
// and may also interfere with string comparison operations.
func removeUnicodeZeroWidthChars(s string) string {
	// List of zero-width Unicode characters to remove
	zeroWidthChars := []string{
		"\u200B", // ZERO WIDTH SPACE
		"\u200C", // ZERO WIDTH NON-JOINER
		"\u200D", // ZERO WIDTH JOINER
		"\u200E", // LEFT-TO-RIGHT MARK
		"\u200F", // RIGHT-TO-LEFT MARK
		"\u2060", // WORD JOINER
		"\u2061", // FUNCTION APPLICATION
		"\u2062", // INVISIBLE TIMES
		"\u2063", // INVISIBLE SEPARATOR
		"\u2064", // INVISIBLE PLUS
		"\uFEFF", // ZERO WIDTH NO-BREAK SPACE
	}

	// Replace all zero-width characters with empty string
	result := s
	for _, char := range zeroWidthChars {
		result = strings.ReplaceAll(result, char, "")
	}

	return result
}

// Public functions (alphabetical)

// GetContainerTitle returns a user-friendly title for the container based on its metadata.
// It follows a hierarchical approach to find the most appropriate title representation
// for displaying to users in interfaces and logs.
func (p *Prober) GetContainerTitle(info *ContainerInfo) string {
	return getContainerTitle(info)
}

// GetExtendedContainerInfo extracts comprehensive metadata about a media file.
// It includes details about all streams (video, audio, subtitles, etc.), chapters,
// and container format. This provides a complete picture of the media file's structure
// and technical properties.
func (p *Prober) GetExtendedContainerInfo(filePath string) (*ContainerInfo, error) {
	// Make sure ffprobe is available
	if p.FFmpegInfo == nil || !p.FFmpegInfo.Installed {
		return nil, fmt.Errorf("ffprobe not available")
	}

	// Get the ffprobe output
	probeOutput, err := p.runFFprobeCommand(filePath)
	if err != nil {
		return nil, err
	}

	// Create and initialize the container info
	containerInfo := p.initializeContainerInfo(probeOutput, filePath)

	// Process streams
	p.processStreams(probeOutput.Streams, containerInfo)

	// Process chapters
	p.processChapters(probeOutput.Chapters, containerInfo)

	return containerInfo, nil
}

// runFFprobeCommand executes the ffprobe command and returns the parsed output.
func (p *Prober) runFFprobeCommand(filePath string) (*ffprobeOutput, error) {
	// Get the path to ffprobe (replace ffmpeg with ffprobe in the path)
	ffprobePath := strings.Replace(p.FFmpegInfo.Path, "ffmpeg", "ffprobe", 1)

	// Create command to get detailed container info
	cmd := exec.Command(
		ffprobePath,
		"-loglevel", "error",
		"-hide_banner",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-show_chapters",
		filePath,
	)

	// Run the command and collect output
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error running ffprobe: %w", err)
	}

	// Parse the JSON output
	var probeOutput ffprobeOutput
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		return nil, fmt.Errorf("error parsing ffprobe JSON output: %w", err)
	}

	return &probeOutput, nil
}

// initializeContainerInfo creates and initializes a ContainerInfo from ffprobe output.
func (p *Prober) initializeContainerInfo(probeOutput *ffprobeOutput, filePath string) *ContainerInfo {
	// Create the container info structure
	containerInfo := &ContainerInfo{
		General: GeneralInfo{
			Format:      probeOutput.Format.FormatName,
			BitRate:     probeOutput.Format.BitRate,
			Duration:    probeOutput.Format.Duration,
			Size:        probeOutput.Format.Size,
			StartTime:   probeOutput.Format.StartTime,
			StreamCount: probeOutput.Format.NBStreams,
			Tags:        make(map[string]string),
		},
		VideoStreams:      []VideoStream{},
		AudioStreams:      []AudioStream{},
		SubtitleStreams:   []SubtitleStream{},
		ChapterStreams:    []ChapterStream{},
		AttachmentStreams: []AttachmentStream{},
		DataStreams:       []DataStream{},
		OtherStreams:      []OtherStream{},
	}

	// Store the file path in the tags
	if probeOutput.Format.Tags == nil {
		probeOutput.Format.Tags = make(map[string]string)
	}
	probeOutput.Format.Tags["file_path"] = filePath
	containerInfo.General.Tags = probeOutput.Format.Tags

	// Parse duration as float
	if probeOutput.Format.Duration != "" {
		durationF, err := strconv.ParseFloat(probeOutput.Format.Duration, 64)
		if err == nil {
			containerInfo.General.DurationF = durationF
		}
	}

	return containerInfo
}

// processStreams handles all streams from the ffprobe output.
func (p *Prober) processStreams(streams []ffprobeStreamOutput, containerInfo *ContainerInfo) {
	for _, stream := range streams {
		// Extract common stream information
		streamInfo := p.extractCommonStreamInfo(stream)

		// Process the stream based on its type
		switch stream.CodecType {
		case "video":
			videoStream := p.processVideoStream(stream, streamInfo)
			containerInfo.VideoStreams = append(containerInfo.VideoStreams, videoStream)

		case "audio":
			audioStream := p.processAudioStream(stream, streamInfo)
			containerInfo.AudioStreams = append(containerInfo.AudioStreams, audioStream)

		case "subtitle":
			subtitleStream := p.processSubtitleStream(stream, streamInfo)
			containerInfo.SubtitleStreams = append(containerInfo.SubtitleStreams, subtitleStream)

		case "attachment":
			attachmentStream := p.processAttachmentStream(stream)
			containerInfo.AttachmentStreams = append(containerInfo.AttachmentStreams, attachmentStream)

		case "data":
			dataStream := p.processDataStream(stream, streamInfo)
			containerInfo.DataStreams = append(containerInfo.DataStreams, dataStream)

		default:
			otherStream := p.processOtherStream(stream, streamInfo)
			containerInfo.OtherStreams = append(containerInfo.OtherStreams, otherStream)
		}
	}
}

// extractCommonStreamInfo extracts common information from a stream.
func (p *Prober) extractCommonStreamInfo(stream ffprobeStreamOutput) StreamInfo {
	// Initialize common stream info
	info := StreamInfo{
		Index:      stream.Index,
		Format:     stream.CodecName,
		FormatFull: stream.CodecLongName,
	}

	// Get title and language from stream tags
	if stream.Tags != nil {
		if t, ok := stream.Tags["title"]; ok {
			info.Title = removeUnicodeZeroWidthChars(t)
		}
		if l, ok := stream.Tags["language"]; ok {
			info.Language = l
		}
	}

	return info
}

// StreamInfo holds common information for different stream types.
type StreamInfo struct {
	Index      int
	Format     string
	FormatFull string
	Title      string
	Language   string
}

// processVideoStream converts ffprobe data to a VideoStream.
func (p *Prober) processVideoStream(stream ffprobeStreamOutput, info StreamInfo) VideoStream {
	// Parse bitrate
	bitRate := p.parseIntField(stream.BitRate)

	// Parse frame rate
	frameRate := p.parseRational(stream.FrameRate)

	// Parse display aspect ratio
	displayAspectRatio := p.parseRational(stream.DisplayAspectRatio)

	// Parse pixel aspect ratio
	pixelAspectRatio := p.parseRational(stream.SampleAspectRatio)

	// Parse bit depth
	bitDepth := 8
	if stream.BitsPerRawSample != "" {
		if bd, err := strconv.Atoi(stream.BitsPerRawSample); err == nil {
			bitDepth = bd
		}
	}

	// Parse duration
	duration := p.parseFloatField(stream.Duration)

	// Create video stream object
	return VideoStream{
		Index:              info.Index,
		Format:             info.Format,
		FormatFull:         info.FormatFull,
		FormatProfile:      stream.Profile,
		Width:              stream.Width,
		Height:             stream.Height,
		DisplayAspectRatio: displayAspectRatio,
		PixelAspectRatio:   pixelAspectRatio,
		FrameRate:          frameRate,
		FrameRateMode:      "Unknown", // Not directly available from FFprobe
		BitRate:            bitRate,
		BitDepth:           bitDepth,
		Duration:           duration,
		ColorSpace:         stream.ColorSpace,
		ScanType:           stream.FieldOrder,
		HasBFrames:         stream.HasBFrames > 0,
		Language:           info.Language,
		Title:              info.Title,
	}
}

// processAudioStream converts ffprobe data to an AudioStream.
func (p *Prober) processAudioStream(stream ffprobeStreamOutput, info StreamInfo) AudioStream {
	// Parse bitrate
	bitRate := p.parseIntField(stream.BitRate)

	// Parse sampling rate
	samplingRate := 0
	if stream.SampleRate != "" {
		if sr, err := strconv.Atoi(stream.SampleRate); err == nil {
			samplingRate = sr
		}
	}

	// Parse duration
	duration := p.parseFloatField(stream.Duration)

	// Create audio stream object
	return AudioStream{
		Index:         info.Index,
		Format:        info.Format,
		FormatFull:    info.FormatFull,
		Channels:      stream.Channels,
		ChannelLayout: stream.ChannelLayout,
		SamplingRate:  samplingRate,
		BitRate:       bitRate,
		Duration:      duration,
		Language:      info.Language,
		Title:         info.Title,
	}
}

// processSubtitleStream converts ffprobe data to a SubtitleStream.
func (p *Prober) processSubtitleStream(stream ffprobeStreamOutput, info StreamInfo) SubtitleStream {
	return SubtitleStream{
		Index:      info.Index,
		Format:     info.Format,
		FormatFull: info.FormatFull,
		Language:   info.Language,
		Title:      info.Title,
	}
}

// processAttachmentStream converts ffprobe data to an AttachmentStream.
func (p *Prober) processAttachmentStream(stream ffprobeStreamOutput) AttachmentStream {
	// Get filename from tags
	fileName := ""
	mimeType := ""
	if stream.Tags != nil {
		if fn, ok := stream.Tags["filename"]; ok {
			fileName = fn
		}
		if mt, ok := stream.Tags["mimetype"]; ok {
			mimeType = mt
		}
	}

	return AttachmentStream{
		Index:    stream.Index,
		FileName: fileName,
		MimeType: mimeType,
	}
}

// processDataStream converts ffprobe data to a DataStream.
func (p *Prober) processDataStream(stream ffprobeStreamOutput, info StreamInfo) DataStream {
	return DataStream{
		Index:      info.Index,
		Format:     info.Format,
		FormatFull: info.FormatFull,
		Title:      info.Title,
	}
}

// processOtherStream converts ffprobe data to an OtherStream.
func (p *Prober) processOtherStream(stream ffprobeStreamOutput, info StreamInfo) OtherStream {
	return OtherStream{
		Index:      info.Index,
		Type:       stream.CodecType,
		Format:     info.Format,
		FormatFull: info.FormatFull,
	}
}

// processChapters handles all chapters from the ffprobe output.
func (p *Prober) processChapters(chapters []chapterOutput, containerInfo *ContainerInfo) {
	for _, chapter := range chapters {
		chapterStream := p.processChapter(chapter)
		containerInfo.ChapterStreams = append(containerInfo.ChapterStreams, chapterStream)
	}
}

// processChapter converts ffprobe data to a ChapterStream.
func (p *Prober) processChapter(chapter chapterOutput) ChapterStream {
	// Parse start and end times
	startTime := p.parseFloatField(chapter.StartTime)
	endTime := p.parseFloatField(chapter.EndTime)

	// Get title from tags
	title := ""
	if chapter.Tags != nil {
		if t, ok := chapter.Tags["title"]; ok {
			title = t
		}
	}

	return ChapterStream{
		ID:        chapter.ID,
		StartTime: startTime,
		EndTime:   endTime,
		Title:     title,
	}
}

// parseIntField converts a string field to an int64.
func (p *Prober) parseIntField(value string) int64 {
	if value == "" {
		return 0
	}

	result, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}

	return result
}

// parseFloatField converts a string field to a float64.
func (p *Prober) parseFloatField(value string) float64 {
	if value == "" {
		return 0
	}

	result, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}

	return result
}

// parseRational converts a string in format "num/den" to a float64.
func (p *Prober) parseRational(value string) float64 {
	if value == "" {
		return 0
	}

	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return 0
	}

	num, errNum := strconv.ParseFloat(parts[0], 64)
	den, errDen := strconv.ParseFloat(parts[1], 64)
	if errNum != nil || errDen != nil || den == 0 {
		return 0
	}

	return num / den
}

// NewProber creates a new Prober instance configured with the provided FFmpeg information.
// It verifies that FFmpeg is properly installed and available for use before
// creating the Prober, preventing operations on an invalid FFmpeg installation.
func NewProber(ffmpegInfo *FFmpegInfo) (*Prober, error) {
	if ffmpegInfo == nil {
		return nil, fmt.Errorf("ffmpeg information cannot be nil")
	}

	if !ffmpegInfo.Installed {
		return nil, fmt.Errorf("ffmpeg is not installed")
	}

	return &Prober{
		FFmpegInfo: ffmpegInfo,
	}, nil
}
