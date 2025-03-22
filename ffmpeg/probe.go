// Package ffmpeg provides functionality for detecting and working with FFmpeg.
package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Private methods (alphabetical)

// processAudioStream processes audio stream data
func (p *Prober) processAudioStream(section string, audioStream *AudioStream) {
	// Split text into lines and iterate
	lines := strings.Split(section, "\n")
	for i, line := range lines {
		if i == 0 && strings.Contains(line, ":") {
			// Process the first line that contains the key-value format
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key, value := parts[0], strings.TrimSpace(parts[1])
				p.processAudioStreamKey(key, value, audioStream)
			}
		} else {
			p.processAudioStreamBasicInfo([]string{line}, audioStream)
			p.processAudioStreamTechnicalInfo([]string{line}, audioStream)
			p.processAudioStreamLanguageInfo([]string{line}, audioStream)
		}
	}
}

// processAudioStreamKey processes a single key-value pair for audio stream
func (p *Prober) processAudioStreamKey(key, value string, audioStream *AudioStream) {
	switch key {
	case "ID", "Format", "Format/Info", "Format profile", "Format settings", "Codec ID", "Language", "Title":
		// Basic metadata fields
		p.processAudioBasicMetadata(key, value, audioStream)
	case "Duration", "Bit rate", "Bit rate mode", "Channel(s)", "Sampling rate", "Bit depth", "Compression mode":
		// Technical fields
		p.processAudioTechnicalMetadata(key, value, audioStream)
	case "Default", "Forced":
		// Flag fields
		p.processAudioFlagMetadata(key, value, audioStream)
	case "Stream size":
		// Size field (special processing)
		p.processAudioStreamSize(value, audioStream)
	}
}

// processAudioBasicMetadata handles basic metadata fields for audio streams
func (p *Prober) processAudioBasicMetadata(key, value string, audioStream *AudioStream) {
	switch key {
	case "ID":
		audioStream.ID = value
	case "Format":
		audioStream.Format = value
	case "Format/Info":
		audioStream.FormatInfo = value
	case "Format profile":
		audioStream.FormatProfile = value
	case "Format settings":
		audioStream.FormatSettings = value
	case "Codec ID":
		audioStream.CodecID = value
	case "Language":
		audioStream.Language = value
	case "Title":
		audioStream.Title = value
	}
}

// processAudioTechnicalMetadata handles technical metadata fields for audio streams
func (p *Prober) processAudioTechnicalMetadata(key, value string, audioStream *AudioStream) {
	switch key {
	case "Duration":
		p.parseAudioDuration("Duration "+value, audioStream)
	case "Bit rate":
		p.parseAudioBitRate("Bit rate "+value, audioStream)
	case "Bit rate mode":
		audioStream.BitRateMode = value
	case "Channel(s)":
		p.parseAudioChannels("Channel(s) "+value, audioStream)
	case "Sampling rate":
		p.parseAudioSamplingRate("Sampling rate "+value, audioStream)
	case "Bit depth":
		p.parseAudioBitDepth("Bit depth "+value, audioStream)
	case "Compression mode":
		audioStream.CompressionMode = value
	}
}

// processAudioFlagMetadata handles flag fields for audio streams
func (p *Prober) processAudioFlagMetadata(key, value string, audioStream *AudioStream) {
	switch key {
	case "Default":
		audioStream.Default = (value == "Yes")
	case "Forced":
		audioStream.Forced = (value == "Yes")
	}
}

// processAudioStreamSize processes the stream size for audio stream
func (p *Prober) processAudioStreamSize(value string, audioStream *AudioStream) {
	// Process stream size similar to bit rate
	sizeStr := "Stream size " + value
	parts := strings.Split(sizeStr, " ")
	if len(parts) > 2 {
		// Parse value and unit, e.g., "2.00 MiB"
		valueStr := strings.ReplaceAll(parts[2], " ", "")
		size, err := strconv.ParseFloat(valueStr, 64)
		if err == nil {
			// Convert to bytes based on unit
			if len(parts) > 3 {
				unit := strings.ToLower(parts[3])
				if strings.HasPrefix(unit, "kib") {
					size *= 1024
				} else if strings.HasPrefix(unit, "mib") {
					size *= 1024 * 1024
				} else if strings.HasPrefix(unit, "gib") {
					size *= 1024 * 1024 * 1024
				} else if strings.HasPrefix(unit, "kb") {
					size *= 1000
				} else if strings.HasPrefix(unit, "mb") {
					size *= 1000000
				} else if strings.HasPrefix(unit, "gb") {
					size *= 1000000000
				}
			}
			audioStream.StreamSize = int64(size)
		}
	}
}

// processAudioStreamBasicInfo processes the basic information from audio stream text lines
func (p *Prober) processAudioStreamBasicInfo(lines []string, audioStream *AudioStream) {
	for _, line := range lines {
		if strings.HasPrefix(line, "ID") {
			audioStream.ID = strings.TrimSpace(strings.TrimPrefix(line, "ID"))
		} else if strings.HasPrefix(line, "Format") {
			audioStream.Format = strings.TrimSpace(strings.TrimPrefix(line, "Format"))
		} else if strings.HasPrefix(line, "Format/Info") {
			audioStream.FormatInfo = strings.TrimSpace(strings.TrimPrefix(line, "Format/Info"))
		} else if strings.HasPrefix(line, "Format profile") {
			audioStream.FormatProfile = strings.TrimSpace(strings.TrimPrefix(line, "Format profile"))
		} else if strings.HasPrefix(line, "Format settings") {
			audioStream.FormatSettings = strings.TrimSpace(strings.TrimPrefix(line, "Format settings"))
		} else if strings.HasPrefix(line, "Codec ID") {
			audioStream.CodecID = strings.TrimSpace(strings.TrimPrefix(line, "Codec ID"))
		}
	}
}

// processAudioStreamTechnicalInfo processes technical details from audio stream text lines
func (p *Prober) processAudioStreamTechnicalInfo(lines []string, audioStream *AudioStream) {
	for _, line := range lines {
		if strings.HasPrefix(line, "Duration") {
			p.parseAudioDuration(line, audioStream)
		} else if strings.HasPrefix(line, "Bit rate") {
			p.parseAudioBitRate(line, audioStream)
		} else if strings.HasPrefix(line, "Channel(s)") {
			p.parseAudioChannels(line, audioStream)
		} else if strings.HasPrefix(line, "Sampling rate") {
			p.parseAudioSamplingRate(line, audioStream)
		} else if strings.HasPrefix(line, "Bit depth") {
			p.parseAudioBitDepth(line, audioStream)
		} else if strings.HasPrefix(line, "Compression mode") {
			audioStream.CompressionMode = strings.TrimSpace(strings.TrimPrefix(line, "Compression mode"))
		}
	}
}

// parseAudioDuration parses the duration string from an audio stream line
func (p *Prober) parseAudioDuration(line string, audioStream *AudioStream) {
	durationStr := strings.TrimSpace(strings.TrimPrefix(line, "Duration"))
	parts := strings.Split(durationStr, " ")
	if len(parts) > 0 {
		// Format is expected to be like: 1h 42mn or like: 1h 42 min, etc.
		// Try to parse to seconds
		durationParts := strings.Split(parts[0], " ")
		seconds := 0.0
		for _, part := range durationParts {
			if strings.Contains(part, "h") {
				hours, err := strconv.ParseFloat(strings.TrimSuffix(part, "h"), 64)
				if err == nil {
					seconds += hours * 3600
				}
			} else if strings.Contains(part, "mn") || strings.Contains(part, "min") {
				minutes, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(part, "mn"), "min"), 64)
				if err == nil {
					seconds += minutes * 60
				}
			} else if strings.Contains(part, "s") {
				secs, err := strconv.ParseFloat(strings.TrimSuffix(part, "s"), 64)
				if err == nil {
					seconds += secs
				}
			}
		}
		audioStream.Duration = seconds
	}
}

// parseAudioBitRate parses the bit rate from an audio stream line
func (p *Prober) parseAudioBitRate(line string, audioStream *AudioStream) {
	bitrateStr := strings.TrimSpace(strings.TrimPrefix(line, "Bit rate"))
	parts := strings.Split(bitrateStr, " ")
	if len(parts) > 0 {
		// Parse value and unit, e.g., "320 kb/s"
		valueStr := strings.ReplaceAll(parts[0], " ", "")
		bitrate, err := strconv.ParseInt(valueStr, 10, 64)
		if err == nil {
			// Convert to bits per second based on unit
			if len(parts) > 1 {
				unit := strings.ToLower(parts[1])
				if strings.HasPrefix(unit, "kb") {
					bitrate *= 1000
				} else if strings.HasPrefix(unit, "mb") {
					bitrate *= 1000000
				} else if strings.HasPrefix(unit, "gb") {
					bitrate *= 1000000000
				}
			}
			audioStream.BitRate = bitrate
		}
	}
}

// parseAudioChannels parses the channel count from an audio stream line
func (p *Prober) parseAudioChannels(line string, audioStream *AudioStream) {
	channelsStr := strings.TrimSpace(strings.TrimPrefix(line, "Channel(s)"))
	parts := strings.Split(channelsStr, " ")
	if len(parts) > 0 {
		channels, err := strconv.Atoi(parts[0])
		if err == nil {
			audioStream.Channels = channels
		}
	}
}

// parseAudioSamplingRate parses the sampling rate from an audio stream line
func (p *Prober) parseAudioSamplingRate(line string, audioStream *AudioStream) {
	rateStr := strings.TrimSpace(strings.TrimPrefix(line, "Sampling rate"))
	parts := strings.Split(rateStr, " ")
	if len(parts) > 0 {
		// Parse value, e.g., "48.0 kHz"
		valueStr := strings.ReplaceAll(parts[0], " ", "")
		rate, err := strconv.ParseFloat(valueStr, 64)
		if err == nil {
			// Convert to Hz based on unit
			if len(parts) > 1 {
				unit := strings.ToLower(parts[1])
				if strings.HasPrefix(unit, "khz") {
					rate *= 1000
				} else if strings.HasPrefix(unit, "mhz") {
					rate *= 1000000
				}
			}
			audioStream.SamplingRate = int(rate)
		}
	}
}

// parseAudioBitDepth parses the bit depth from an audio stream line
func (p *Prober) parseAudioBitDepth(line string, audioStream *AudioStream) {
	bitDepthStr := strings.TrimSpace(strings.TrimPrefix(line, "Bit depth"))
	parts := strings.Split(bitDepthStr, " ")
	if len(parts) > 0 {
		bitDepth, err := strconv.Atoi(parts[0])
		if err == nil {
			audioStream.BitDepth = bitDepth
		}
	}
}

// processAudioStreamLanguageInfo processes language information from audio stream text lines
func (p *Prober) processAudioStreamLanguageInfo(lines []string, audioStream *AudioStream) {
	for _, line := range lines {
		if strings.HasPrefix(line, "Language") {
			audioStream.Language = strings.TrimSpace(strings.TrimPrefix(line, "Language"))
		} else if strings.HasPrefix(line, "Title") {
			audioStream.Title = strings.TrimSpace(strings.TrimPrefix(line, "Title"))
		} else if strings.HasPrefix(line, "Default") {
			audioStream.Default = (strings.TrimSpace(strings.TrimPrefix(line, "Default")) == "Yes")
		} else if strings.HasPrefix(line, "Forced") {
			audioStream.Forced = (strings.TrimSpace(strings.TrimPrefix(line, "Forced")) == "Yes")
		}
	}
}

// processGeneralInfo processes general information from a section
func (p *Prober) processGeneralInfo(section string, generalInfo *GeneralInfo) {
	// Split text into lines and iterate
	lines := strings.Split(section, "\n")
	for i, line := range lines {
		if i == 0 && strings.Contains(line, ":") {
			// Process the first line that contains the key-value format
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key, value := parts[0], strings.TrimSpace(parts[1])
				p.processGeneralInfoKey(key, value, generalInfo)
			}
		} else {
			if strings.HasPrefix(line, "Complete name") ||
				strings.HasPrefix(line, "Format") ||
				strings.HasPrefix(line, "Format version") ||
				strings.HasPrefix(line, "Unique ID") ||
				strings.HasPrefix(line, "Encoded date") ||
				strings.HasPrefix(line, "Writing application") ||
				strings.HasPrefix(line, "Writing library") {
				p.processGeneralBasicInfoLine(line, generalInfo)
			} else {
				p.processGeneralFileSize([]string{line}, generalInfo)
				p.processGeneralDuration([]string{line}, generalInfo)
				p.processGeneralBitRate([]string{line}, generalInfo)
				p.processGeneralFrameRate([]string{line}, generalInfo)
			}
		}
	}
}

// processGeneralInfoKey processes a single key-value pair for general info
func (p *Prober) processGeneralInfoKey(key, value string, generalInfo *GeneralInfo) {
	switch key {
	case "Complete name":
		generalInfo.CompleteName = value
	case "Format":
		generalInfo.Format = value
	case "Format version":
		generalInfo.FormatVersion = value
	case "File size":
		p.processGeneralFileSize([]string{"File size " + value}, generalInfo)
	case "Duration":
		p.processGeneralDuration([]string{"Duration " + value}, generalInfo)
	case "Overall bit rate":
		p.processGeneralBitRate([]string{"Overall bit rate " + value}, generalInfo)
	case "Frame rate":
		p.processGeneralFrameRate([]string{"Frame rate " + value}, generalInfo)
	case "Encoded date":
		generalInfo.EncodedDate = value
	case "Writing application":
		generalInfo.WritingApplication = value
	case "Writing library":
		generalInfo.WritingLibrary = value
	}
}

// processGeneralBasicInfoLine processes basic information lines for general info
func (p *Prober) processGeneralBasicInfoLine(line string, generalInfo *GeneralInfo) {
	if strings.HasPrefix(line, "Complete name") {
		generalInfo.CompleteName = strings.TrimSpace(strings.TrimPrefix(line, "Complete name"))
	} else if strings.HasPrefix(line, "Format") && !strings.HasPrefix(line, "Format/") {
		generalInfo.Format = strings.TrimSpace(strings.TrimPrefix(line, "Format"))
	} else if strings.HasPrefix(line, "Format version") {
		generalInfo.FormatVersion = strings.TrimSpace(strings.TrimPrefix(line, "Format version"))
	} else if strings.HasPrefix(line, "Unique ID") {
		generalInfo.UniqueID = strings.TrimSpace(strings.TrimPrefix(line, "Unique ID"))
	} else if strings.HasPrefix(line, "Encoded date") {
		generalInfo.EncodedDate = strings.TrimSpace(strings.TrimPrefix(line, "Encoded date"))
	} else if strings.HasPrefix(line, "Writing application") {
		generalInfo.WritingApplication = strings.TrimSpace(strings.TrimPrefix(line, "Writing application"))
	} else if strings.HasPrefix(line, "Writing library") {
		generalInfo.WritingLibrary = strings.TrimSpace(strings.TrimPrefix(line, "Writing library"))
	}
}

// processGeneralFileSize processes the File size field of GeneralInfo
func (p *Prober) processGeneralFileSize(lines []string, generalInfo *GeneralInfo) {
	fileSizeText := lines[0]
	// Trim prefix if present
	if strings.HasPrefix(fileSizeText, "File size") {
		fileSizeText = strings.TrimSpace(strings.TrimPrefix(fileSizeText, "File size"))
	}

	// Parse file size (e.g., "1.23 GiB")
	parts := strings.Fields(fileSizeText)
	if len(parts) >= 1 {
		size, err := strconv.ParseFloat(parts[0], 64)
		if err == nil {
			// Convert to bytes based on unit
			if len(parts) > 1 {
				unit := strings.ToLower(parts[1])
				switch {
				case strings.HasPrefix(unit, "kib"):
					size *= 1024
				case strings.HasPrefix(unit, "mib"):
					size *= 1024 * 1024
				case strings.HasPrefix(unit, "gib"):
					size *= 1024 * 1024 * 1024
				case strings.HasPrefix(unit, "tib"):
					size *= 1024 * 1024 * 1024 * 1024
				case strings.HasPrefix(unit, "kb"):
					size *= 1000
				case strings.HasPrefix(unit, "mb"):
					size *= 1000 * 1000
				case strings.HasPrefix(unit, "gb"):
					size *= 1000 * 1000 * 1000
				case strings.HasPrefix(unit, "tb"):
					size *= 1000 * 1000 * 1000 * 1000
				}
			}
			generalInfo.FileSize = int64(size)
		}
	}
}

// processGeneralDuration processes the Duration field of GeneralInfo
func (p *Prober) processGeneralDuration(lines []string, generalInfo *GeneralInfo) {
	durationText := lines[0]
	// Trim prefix if present
	if strings.HasPrefix(durationText, "Duration") {
		durationText = strings.TrimSpace(strings.TrimPrefix(durationText, "Duration"))
	}

	// Parse duration format (e.g., "28min 18s" or "28 min 18 s")
	// First, try to parse hours, minutes, seconds format
	parts := strings.Fields(durationText)
	seconds := 0.0

	for _, part := range parts {
		if strings.Contains(part, "h") {
			hours, err := strconv.ParseFloat(strings.TrimSuffix(part, "h"), 64)
			if err == nil {
				seconds += hours * 3600
			}
		} else if strings.Contains(part, "min") {
			minutes, err := strconv.ParseFloat(strings.TrimSuffix(part, "min"), 64)
			if err == nil {
				seconds += minutes * 60
			}
		} else if strings.Contains(part, "s") {
			secs, err := strconv.ParseFloat(strings.TrimSuffix(part, "s"), 64)
			if err == nil {
				seconds += secs
			}
		}
	}

	generalInfo.Duration = seconds
}

// processGeneralBitRate processes the Bit rate field of GeneralInfo
func (p *Prober) processGeneralBitRate(lines []string, generalInfo *GeneralInfo) {
	bitrateText := lines[0]
	// Trim prefix if present
	if strings.HasPrefix(bitrateText, "Overall bit rate") {
		bitrateText = strings.TrimSpace(strings.TrimPrefix(bitrateText, "Overall bit rate"))
	}

	// Parse bit rate (e.g., "5 000 kb/s")
	parts := strings.Fields(bitrateText)
	if len(parts) >= 1 {
		// Handle formats like "5 000" by removing spaces
		valueStr := strings.ReplaceAll(parts[0], " ", "")
		bitrate, err := strconv.ParseInt(valueStr, 10, 64)
		if err == nil {
			// Convert to bits per second based on unit
			if len(parts) > 1 {
				unit := strings.ToLower(parts[1])
				if strings.HasPrefix(unit, "kb") {
					bitrate *= 1000
				} else if strings.HasPrefix(unit, "mb") {
					bitrate *= 1000000
				} else if strings.HasPrefix(unit, "gb") {
					bitrate *= 1000000000
				}
			}
			generalInfo.OverallBitRate = bitrate
		}
	}
}

// processGeneralFrameRate processes the Frame rate field of GeneralInfo
func (p *Prober) processGeneralFrameRate(lines []string, generalInfo *GeneralInfo) {
	rateText := lines[0]
	// Trim prefix if present
	if strings.HasPrefix(rateText, "Frame rate") {
		rateText = strings.TrimSpace(strings.TrimPrefix(rateText, "Frame rate"))
	}

	// Parse frame rate (e.g., "23.976 FPS")
	parts := strings.Fields(rateText)
	if len(parts) >= 1 {
		rate, err := strconv.ParseFloat(parts[0], 64)
		if err == nil {
			generalInfo.FrameRate = rate
		}
	}
}

// processJSONAudioStream processes JSON audio stream data
func (p *Prober) processJSONAudioStream(stream map[string]interface{}, audioStream *AudioStream) {
	p.processJSONAudioBasicInfo(stream, audioStream)
	p.processJSONAudioRates(stream, audioStream)
	p.processJSONAudioChannelInfo(stream, audioStream)

	// Extract tags if available
	if tags, ok := stream["tags"].(map[string]interface{}); ok {
		p.processJSONAudioTags(audioStream, tags)
	}
}

// processJSONAudioBasicInfo extracts basic information from JSON audio stream data
func (p *Prober) processJSONAudioBasicInfo(stream map[string]interface{}, audioStream *AudioStream) {
	// Extract stream index
	if index, ok := stream["index"].(float64); ok {
		audioStream.ID = fmt.Sprintf("%d", int(index))
	}

	// Extract codec name
	if codecName, ok := stream["codec_name"].(string); ok {
		audioStream.Format = codecName
	}

	// Extract codec long name
	if codecLongName, ok := stream["codec_long_name"].(string); ok {
		audioStream.FormatInfo = codecLongName
	}

	// Extract codec tag
	if codecTag, ok := stream["codec_tag"].(string); ok {
		audioStream.CodecID = codecTag
	}

	// Extract duration
	if durationStr, ok := stream["duration"].(string); ok {
		duration, err := strconv.ParseFloat(durationStr, 64)
		if err == nil {
			audioStream.Duration = duration
		}
	}
}

// processJSONAudioRates extracts rate information from JSON audio stream data
func (p *Prober) processJSONAudioRates(stream map[string]interface{}, audioStream *AudioStream) {
	// Extract bit rate
	if bitrateStr, ok := stream["bit_rate"].(string); ok {
		bitrate, err := strconv.ParseInt(bitrateStr, 10, 64)
		if err == nil {
			audioStream.BitRate = bitrate
		}
	}

	// Extract sampling rate
	if sampleRateStr, ok := stream["sample_rate"].(string); ok {
		sampleRate, err := strconv.ParseFloat(sampleRateStr, 64)
		if err == nil {
			audioStream.SamplingRate = int(sampleRate)
		}
	}
}

// processJSONAudioChannelInfo extracts channel information from JSON audio stream data
func (p *Prober) processJSONAudioChannelInfo(stream map[string]interface{}, audioStream *AudioStream) {
	// Extract channels
	if channels, ok := stream["channels"].(float64); ok {
		audioStream.Channels = int(channels)
	}

	// Extract channel layout
	if channelLayout, ok := stream["channel_layout"].(string); ok {
		audioStream.ChannelLayout = channelLayout
	}
}

// processJSONAudioTags processes the tags section of JSON audio stream data
func (p *Prober) processJSONAudioTags(audioStream *AudioStream, tags map[string]interface{}) {
	// Extract language
	if language, ok := tags["language"].(string); ok {
		audioStream.Language = language
	}

	// Extract title
	if title, ok := tags["title"].(string); ok {
		audioStream.Title = title
	}

	// Extract default flag
	if defaultFlag, ok := tags["DISPOSITION:default"].(string); ok {
		audioStream.Default = (defaultFlag == "1")
	}

	// Extract forced flag
	if forcedFlag, ok := tags["DISPOSITION:forced"].(string); ok {
		audioStream.Forced = (forcedFlag == "1")
	}
}

// processJSONFormat processes JSON format data
func (p *Prober) processJSONFormat(format map[string]interface{}, info *GeneralInfo) {
	// Extract format name
	if formatName, ok := format["format_name"].(string); ok {
		info.Format = formatName
	}

	// Extract format long name
	if formatLongName, ok := format["format_long_name"].(string); ok {
		info.FormatVersion = formatLongName
	}

	// Extract filename and determine extension
	if filename, ok := format["filename"].(string); ok {
		ext := filepath.Ext(filename)
		if ext != "" {
			// Remove the dot from the extension
			ext = ext[1:]
			// Add the file extension to the format information
			if info.Format != "" {
				info.Format = fmt.Sprintf("%s (File extension: %s)", info.Format, ext)
			}
		}
	}

	// Extract duration
	if durationStr, ok := format["duration"].(string); ok {
		duration, err := strconv.ParseFloat(durationStr, 64)
		if err == nil {
			info.Duration = duration
		}
	}

	// Extract bit rate
	if bitrateStr, ok := format["bit_rate"].(string); ok {
		bitrate, err := strconv.ParseInt(bitrateStr, 10, 64)
		if err == nil {
			info.OverallBitRate = bitrate
		}
	}

	// Extract file size
	if sizeStr, ok := format["size"].(string); ok {
		size, err := strconv.ParseInt(sizeStr, 10, 64)
		if err == nil {
			info.FileSize = size
		}
	}

	// Extract tags if available
	if tags, ok := format["tags"].(map[string]interface{}); ok {
		// Extract encoder
		if encoder, ok := tags["encoder"].(string); ok {
			info.WritingApplication = encoder
		}

		// Extract creation time
		if creationTime, ok := tags["creation_time"].(string); ok {
			info.EncodedDate = creationTime
		}
	}
}

// processJSONSubtitleStream processes JSON subtitle stream data
func (p *Prober) processJSONSubtitleStream(stream map[string]interface{}, subtitleStream *SubtitleStream) {
	// Extract stream index
	if index, ok := stream["index"].(float64); ok {
		subtitleStream.ID = fmt.Sprintf("%d", int(index))
	}

	// Extract codec name
	if codecName, ok := stream["codec_name"].(string); ok {
		subtitleStream.Format = codecName
	}

	// Extract codec tag
	if codecTag, ok := stream["codec_tag"].(string); ok {
		subtitleStream.CodecID = codecTag
	}

	// Extract codec long name
	if codecLongName, ok := stream["codec_long_name"].(string); ok {
		subtitleStream.CodecIDInfo = codecLongName
	}

	// Extract duration
	if durationStr, ok := stream["duration"].(string); ok {
		duration, err := strconv.ParseFloat(durationStr, 64)
		if err == nil {
			subtitleStream.Duration = duration
		}
	}

	// Extract tags if available
	if tags, ok := stream["tags"].(map[string]interface{}); ok {
		// Extract language
		if language, ok := tags["language"].(string); ok {
			subtitleStream.Language = language
		}

		// Extract title
		if title, ok := tags["title"].(string); ok {
			subtitleStream.Title = title
		}

		// Extract default flag
		if defaultFlag, ok := tags["DISPOSITION:default"].(string); ok {
			subtitleStream.Default = (defaultFlag == "1")
		}

		// Extract forced flag
		if forcedFlag, ok := tags["DISPOSITION:forced"].(string); ok {
			subtitleStream.Forced = (forcedFlag == "1")
		}
	}
}

// processJSONVideoStream processes JSON video stream data
func (p *Prober) processJSONVideoStream(stream map[string]interface{}, videoStream *VideoStream) {
	p.processJSONVideoBasicInfo(stream, videoStream)
	p.processJSONVideoDimensions(stream, videoStream)
	p.processJSONVideoRates(stream, videoStream)
	p.processJSONVideoQuality(stream, videoStream)

	// Extract tags if available
	if tags, ok := stream["tags"].(map[string]interface{}); ok {
		p.processJSONVideoTags(videoStream, tags)
	}
}

// processJSONVideoBasicInfo extracts basic information from JSON video stream data
func (p *Prober) processJSONVideoBasicInfo(stream map[string]interface{}, videoStream *VideoStream) {
	// Extract stream index
	if index, ok := stream["index"].(float64); ok {
		videoStream.ID = fmt.Sprintf("%d", int(index))
	}

	// Extract codec name
	if codecName, ok := stream["codec_name"].(string); ok {
		videoStream.Format = codecName
	}

	// Extract codec long name
	if codecLongName, ok := stream["codec_long_name"].(string); ok {
		videoStream.FormatInfo = codecLongName
	}

	// Extract codec tag
	if codecTag, ok := stream["codec_tag"].(string); ok {
		videoStream.CodecID = codecTag
	}

	// Extract duration
	if durationStr, ok := stream["duration"].(string); ok {
		duration, err := strconv.ParseFloat(durationStr, 64)
		if err == nil {
			videoStream.Duration = duration
		}
	}
}

// processJSONVideoDimensions extracts dimension information from JSON video stream data
func (p *Prober) processJSONVideoDimensions(stream map[string]interface{}, videoStream *VideoStream) {
	// Extract width
	if width, ok := stream["width"].(float64); ok {
		videoStream.Width = int(width)
	}

	// Extract height
	if height, ok := stream["height"].(float64); ok {
		videoStream.Height = int(height)
	}

	// Extract display aspect ratio
	if dar, ok := stream["display_aspect_ratio"].(string); ok {
		p.parseJSONAspectRatio(videoStream, dar)
	}
}

// parseJSONAspectRatio parses the aspect ratio string into a float value
func (p *Prober) parseJSONAspectRatio(videoStream *VideoStream, dar string) {
	parts := strings.Split(dar, ":")
	if len(parts) == 2 {
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && den > 0 {
			videoStream.DisplayAspectRatio = num / den
			videoStream.AspectRatio = dar // Store the original string format too
		}
	}
}

// processJSONVideoRates extracts bitrate and framerate from JSON video stream data
func (p *Prober) processJSONVideoRates(stream map[string]interface{}, videoStream *VideoStream) {
	// Extract bit rate
	if bitrateStr, ok := stream["bit_rate"].(string); ok {
		bitrate, err := strconv.ParseInt(bitrateStr, 10, 64)
		if err == nil {
			videoStream.BitRate = bitrate
		}
	}

	// Extract frame rate
	if fpsStr, ok := stream["r_frame_rate"].(string); ok {
		p.parseJSONFrameRate(videoStream, fpsStr)
	}
}

// parseJSONFrameRate parses the frame rate string into a float value
func (p *Prober) parseJSONFrameRate(videoStream *VideoStream, fpsStr string) {
	parts := strings.Split(fpsStr, "/")
	if len(parts) == 2 {
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && den > 0 {
			videoStream.FrameRate = num / den
		}
	}
}

// processJSONVideoQuality extracts quality-related information from JSON video stream data
func (p *Prober) processJSONVideoQuality(stream map[string]interface{}, videoStream *VideoStream) {
	// Extract bit depth from profile
	if profile, ok := stream["profile"].(string); ok {
		p.parseJSONBitDepth(videoStream, profile)
	}

	// Extract color space
	if colorSpace, ok := stream["color_space"].(string); ok {
		videoStream.ColorSpace = colorSpace
	}

	// Extract chroma subsampling
	if chromaLocation, ok := stream["chroma_location"].(string); ok {
		videoStream.ChromaSubsampling = chromaLocation
	}
}

// parseJSONBitDepth determines the bit depth from the profile string
func (p *Prober) parseJSONBitDepth(videoStream *VideoStream, profile string) {
	if strings.Contains(profile, "10") {
		videoStream.BitDepth = 10
	} else if strings.Contains(profile, "12") {
		videoStream.BitDepth = 12
	} else {
		videoStream.BitDepth = 8
	}
}

// processJSONVideoTags processes the tags section of JSON video stream data
func (p *Prober) processJSONVideoTags(videoStream *VideoStream, tags map[string]interface{}) {
	// Extract language
	if language, ok := tags["language"].(string); ok {
		videoStream.Language = language
	}

	// Extract title
	if title, ok := tags["title"].(string); ok {
		videoStream.Title = title
	}

	// Extract default flag
	if defaultFlag, ok := tags["DISPOSITION:default"].(string); ok {
		videoStream.Default = (defaultFlag == "1")
	}

	// Extract forced flag
	if forcedFlag, ok := tags["DISPOSITION:forced"].(string); ok {
		videoStream.Forced = (forcedFlag == "1")
	}
}

// processSubtitleStream processes subtitle stream data
func (p *Prober) processSubtitleStream(section string, subtitleStream *SubtitleStream) {
	// Split text into lines and iterate
	lines := strings.Split(section, "\n")
	for i, line := range lines {
		if i == 0 && strings.Contains(line, ":") {
			// Process the first line that contains the key-value format
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				key, value := parts[0], strings.TrimSpace(parts[1])
				p.processSubtitleStreamKey(key, value, subtitleStream)
			}
		} else {
			// Process each line by extracting key-value
			p.processSubtitleStreamLine(line, subtitleStream)
		}
	}
}

// processSubtitleStreamLine processes a single line from subtitle stream text
func (p *Prober) processSubtitleStreamLine(line string, subtitleStream *SubtitleStream) {
	// Identify which type of line we're dealing with
	switch {
	case strings.HasPrefix(line, "ID"):
		subtitleStream.ID = strings.TrimSpace(strings.TrimPrefix(line, "ID"))
	case strings.HasPrefix(line, "Format"):
		subtitleStream.Format = strings.TrimSpace(strings.TrimPrefix(line, "Format"))
	case strings.HasPrefix(line, "Codec ID") && !strings.HasPrefix(line, "Codec ID/Info"):
		subtitleStream.CodecID = strings.TrimSpace(strings.TrimPrefix(line, "Codec ID"))
	case strings.HasPrefix(line, "Codec ID/Info"):
		subtitleStream.CodecIDInfo = strings.TrimSpace(strings.TrimPrefix(line, "Codec ID/Info"))
	case strings.HasPrefix(line, "Duration"):
		p.processSubtitleDuration(line, subtitleStream)
	case strings.HasPrefix(line, "Bit rate"):
		p.processSubtitleBitRate(line, subtitleStream)
	case strings.HasPrefix(line, "Count of elements"):
		countStr := strings.TrimSpace(strings.TrimPrefix(line, "Count of elements"))
		count, err := strconv.Atoi(countStr)
		if err == nil {
			subtitleStream.CountOfElements = count
		}
	case strings.HasPrefix(line, "Language"):
		subtitleStream.Language = strings.TrimSpace(strings.TrimPrefix(line, "Language"))
	case strings.HasPrefix(line, "Title"):
		subtitleStream.Title = strings.TrimSpace(strings.TrimPrefix(line, "Title"))
	case strings.HasPrefix(line, "Default"):
		subtitleStream.Default = (strings.TrimSpace(strings.TrimPrefix(line, "Default")) == "Yes")
	case strings.HasPrefix(line, "Forced"):
		subtitleStream.Forced = (strings.TrimSpace(strings.TrimPrefix(line, "Forced")) == "Yes")
	}
}

// processSubtitleStreamKey processes a single key-value pair for subtitle stream
func (p *Prober) processSubtitleStreamKey(key, value string, subtitleStream *SubtitleStream) {
	switch key {
	case "ID", "Format", "Codec ID", "Codec ID/Info", "Language", "Title":
		// Basic metadata fields
		p.processSubtitleBasicMetadata(key, value, subtitleStream)
	case "Duration", "Bit rate", "Count of elements":
		// Technical fields
		p.processSubtitleTechnicalMetadata(key, value, subtitleStream)
	case "Default", "Forced":
		// Flag fields
		p.processSubtitleFlagMetadata(key, value, subtitleStream)
	}
}

// processSubtitleBasicMetadata handles basic metadata fields for subtitle streams
func (p *Prober) processSubtitleBasicMetadata(key, value string, subtitleStream *SubtitleStream) {
	switch key {
	case "ID":
		subtitleStream.ID = value
	case "Format":
		subtitleStream.Format = value
	case "Codec ID":
		subtitleStream.CodecID = value
	case "Codec ID/Info":
		subtitleStream.CodecIDInfo = value
	case "Language":
		subtitleStream.Language = value
	case "Title":
		subtitleStream.Title = value
	}
}

// processSubtitleTechnicalMetadata handles technical metadata fields for subtitle streams
func (p *Prober) processSubtitleTechnicalMetadata(key, value string, subtitleStream *SubtitleStream) {
	switch key {
	case "Duration":
		p.processSubtitleDuration("Duration "+value, subtitleStream)
	case "Bit rate":
		p.processSubtitleBitRate("Bit rate "+value, subtitleStream)
	case "Count of elements":
		count, err := strconv.Atoi(value)
		if err == nil {
			subtitleStream.CountOfElements = count
		}
	}
}

// processSubtitleFlagMetadata handles flag fields for subtitle streams
func (p *Prober) processSubtitleFlagMetadata(key, value string, subtitleStream *SubtitleStream) {
	switch key {
	case "Default":
		subtitleStream.Default = (value == "Yes")
	case "Forced":
		subtitleStream.Forced = (value == "Yes")
	}
}

// processSubtitleDuration processes the duration from a subtitle stream line
func (p *Prober) processSubtitleDuration(line string, subtitleStream *SubtitleStream) {
	durationStr := strings.TrimSpace(strings.TrimPrefix(line, "Duration"))
	parts := strings.Split(durationStr, " ")
	if len(parts) > 0 {
		// Format is expected to be like: 1h 30min
		// Try to parse to seconds
		seconds := 0.0
		for _, part := range parts {
			if strings.Contains(part, "h") {
				hours, err := strconv.ParseFloat(strings.TrimSuffix(part, "h"), 64)
				if err == nil {
					seconds += hours * 3600
				}
			} else if strings.Contains(part, "min") || strings.Contains(part, "mn") {
				minutes, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(part, "min"), "mn"), 64)
				if err == nil {
					seconds += minutes * 60
				}
			} else if strings.Contains(part, "s") {
				secs, err := strconv.ParseFloat(strings.TrimSuffix(part, "s"), 64)
				if err == nil {
					seconds += secs
				}
			}
		}
		subtitleStream.Duration = seconds
	}
}

// processSubtitleBitRate processes the bit rate from a subtitle stream line
func (p *Prober) processSubtitleBitRate(line string, subtitleStream *SubtitleStream) {
	bitrateStr := strings.TrimSpace(strings.TrimPrefix(line, "Bit rate"))
	parts := strings.Split(bitrateStr, " ")
	if len(parts) > 0 {
		// Parse value and unit, e.g., "3600 b/s"
		valueStr := strings.ReplaceAll(parts[0], " ", "")
		bitrate, err := strconv.ParseInt(valueStr, 10, 64)
		if err == nil {
			// Convert to bits per second based on unit
			if len(parts) > 1 {
				unit := strings.ToLower(parts[1])
				if strings.HasPrefix(unit, "kb") {
					bitrate *= 1000
				} else if strings.HasPrefix(unit, "mb") {
					bitrate *= 1000000
				} else if strings.HasPrefix(unit, "gb") {
					bitrate *= 1000000000
				}
			}
			subtitleStream.BitRate = bitrate
		}
	}
}

// processVideoStream processes video stream data
func (p *Prober) processVideoStream(section string, videoStream *VideoStream) {
	// Split text into lines and process them
	lines := strings.Split(section, "\n")

	// Process each line
	for i, line := range lines {
		p.processVideoStreamLine(i, line, videoStream)
	}
}

// processVideoStreamLine processes a single line from video stream text
func (p *Prober) processVideoStreamLine(lineIndex int, line string, videoStream *VideoStream) {
	// First line processing
	if lineIndex == 0 && strings.Contains(line, ":") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key, value := parts[0], strings.TrimSpace(parts[1])
			p.processVideoStreamKey(key, value, videoStream)
		}
		return
	}

	// Process other lines as key-value pairs if they contain a recognizable prefix
	p.processVideoContentLine(line, videoStream)
}

// processVideoContentLine processes content lines from video stream data
func (p *Prober) processVideoContentLine(line string, videoStream *VideoStream) {
	switch {
	// Basic info fields
	case strings.HasPrefix(line, "ID"):
		videoStream.ID = strings.TrimSpace(strings.TrimPrefix(line, "ID"))
	case strings.HasPrefix(line, "Format"):
		p.processVideoFormatLine(line, videoStream)

	// Codec related fields
	case strings.HasPrefix(line, "Codec ID"):
		p.processVideoCodecLine(line, videoStream)

	// Dimensional properties
	case strings.HasPrefix(line, "Width"):
		p.processVideoDimensionLine(line, "Width", videoStream)
	case strings.HasPrefix(line, "Height"):
		p.processVideoDimensionLine(line, "Height", videoStream)

	// Aspect ratio fields
	case strings.HasPrefix(line, "Display aspect ratio"):
		p.processVideoAspectRatioLine(line, videoStream)

	// Frame rate fields
	case strings.HasPrefix(line, "Frame rate"):
		p.processVideoFrameRateLine(line, videoStream)

	// Other technical fields
	case strings.HasPrefix(line, "Bit depth"):
		p.processVideoBitDepthLine(line, videoStream)
	case strings.HasPrefix(line, "Color space"):
		videoStream.ColorSpace = strings.TrimSpace(strings.TrimPrefix(line, "Color space"))
	case strings.HasPrefix(line, "Chroma subsampling"):
		videoStream.ChromaSubsampling = strings.TrimSpace(strings.TrimPrefix(line, "Chroma subsampling"))

	// Flag fields
	case strings.HasPrefix(line, "Default"):
		videoStream.Default = strings.TrimSpace(strings.TrimPrefix(line, "Default")) == "Yes"
	case strings.HasPrefix(line, "Forced"):
		videoStream.Forced = strings.TrimSpace(strings.TrimPrefix(line, "Forced")) == "Yes"
	}
}

// processVideoFormatLine processes format-related lines
func (p *Prober) processVideoFormatLine(line string, videoStream *VideoStream) {
	if strings.HasPrefix(line, "Format/Info") {
		videoStream.FormatInfo = strings.TrimSpace(strings.TrimPrefix(line, "Format/Info"))
	} else if strings.HasPrefix(line, "Format profile") {
		videoStream.FormatProfile = strings.TrimSpace(strings.TrimPrefix(line, "Format profile"))
	} else if strings.HasPrefix(line, "Format settings") && strings.HasPrefix(line, "Format settings, CABAC") {
		videoStream.FormatSettingsCABAC = strings.TrimSpace(strings.TrimPrefix(line, "Format settings, CABAC"))
	} else if strings.HasPrefix(line, "Format settings") && strings.HasPrefix(line, "Format settings, Reference frames") {
		refFramesStr := strings.TrimSpace(strings.TrimPrefix(line, "Format settings, Reference frames"))
		if refFrames, err := strconv.Atoi(refFramesStr); err == nil {
			videoStream.FormatSettingsRefFrames = refFrames
		}
	} else if strings.HasPrefix(line, "Format") {
		videoStream.Format = strings.TrimSpace(strings.TrimPrefix(line, "Format"))
	}
}

// processVideoCodecLine processes codec-related lines
func (p *Prober) processVideoCodecLine(line string, videoStream *VideoStream) {
	if strings.HasPrefix(line, "Codec ID/Info") {
		videoStream.CodecIDInfo = strings.TrimSpace(strings.TrimPrefix(line, "Codec ID/Info"))
	} else if strings.HasPrefix(line, "Codec ID/Hint") {
		videoStream.CodecIDHint = strings.TrimSpace(strings.TrimPrefix(line, "Codec ID/Hint"))
	} else if strings.HasPrefix(line, "Codec ID") {
		videoStream.CodecID = strings.TrimSpace(strings.TrimPrefix(line, "Codec ID"))
	}
}

// processVideoDimensionLine processes width and height lines
func (p *Prober) processVideoDimensionLine(line string, dimension string, videoStream *VideoStream) {
	dimensionStr := strings.TrimSpace(strings.TrimPrefix(line, dimension))
	parts := strings.Split(dimensionStr, " ")

	if len(parts) > 0 {
		// Extract numeric part
		numericPart := parts[0]
		// Remove any non-numeric characters
		numericPart = strings.TrimSuffix(numericPart, "pixels")
		numericPart = strings.TrimSpace(numericPart)

		if value, err := strconv.Atoi(numericPart); err == nil {
			if dimension == "Width" {
				videoStream.Width = value
			} else if dimension == "Height" {
				videoStream.Height = value
			}
		}
	}
}

// processVideoAspectRatioLine processes display aspect ratio lines
func (p *Prober) processVideoAspectRatioLine(line string, videoStream *VideoStream) {
	aspectRatio := strings.TrimSpace(strings.TrimPrefix(line, "Display aspect ratio"))

	// Store the raw aspect ratio string
	videoStream.AspectRatio = aspectRatio

	// Check for ratio format (e.g., "16:9")
	if strings.Contains(aspectRatio, ":") {
		ratioParts := strings.Split(aspectRatio, ":")
		if len(ratioParts) == 2 {
			if numerator, err := strconv.ParseFloat(ratioParts[0], 64); err == nil {
				if denominator, err := strconv.ParseFloat(ratioParts[1], 64); err == nil && denominator > 0 {
					videoStream.DisplayAspectRatio = numerator / denominator
				}
			}
		}
	} else {
		// Try to parse as float
		if ratio, err := strconv.ParseFloat(aspectRatio, 64); err == nil {
			videoStream.DisplayAspectRatio = ratio
		}
	}
}

// processVideoFrameRateLine processes frame rate lines
func (p *Prober) processVideoFrameRateLine(line string, videoStream *VideoStream) {
	frameRateStr := strings.TrimSpace(strings.TrimPrefix(line, "Frame rate"))

	// Extract numeric part
	numericPart := frameRateStr
	if strings.Contains(frameRateStr, " ") {
		parts := strings.Split(frameRateStr, " ")
		numericPart = parts[0]
	}

	// Remove FPS suffix if present
	numericPart = strings.TrimSuffix(numericPart, "FPS")
	numericPart = strings.TrimSuffix(numericPart, "fps")
	numericPart = strings.TrimSpace(numericPart)

	if frameRate, err := strconv.ParseFloat(numericPart, 64); err == nil {
		videoStream.FrameRate = frameRate
	}
}

// processVideoBitDepthLine processes bit depth lines
func (p *Prober) processVideoBitDepthLine(line string, videoStream *VideoStream) {
	bitDepthStr := strings.TrimSpace(strings.TrimPrefix(line, "Bit depth"))

	// Extract numeric part
	numericPart := bitDepthStr
	if strings.Contains(bitDepthStr, " ") {
		parts := strings.Split(bitDepthStr, " ")
		numericPart = parts[0]
	}

	// Remove 'bits' suffix if present
	numericPart = strings.TrimSuffix(numericPart, "bits")
	numericPart = strings.TrimSpace(numericPart)

	if bitDepth, err := strconv.Atoi(numericPart); err == nil {
		videoStream.BitDepth = bitDepth
	}
}

// Public methods (alphabetical)

// GetExtendedContainerInfo runs ffprobe on the given file and returns detailed container information
func (p *Prober) GetExtendedContainerInfo(filePath string) (*ContainerInfo, error) {
	// Get absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path: %w", err)
	}

	// Run ffprobe with JSON output format
	cmd := exec.Command(p.FFprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		absPath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error running ffprobe: %w", err)
	}

	// If we got no output, try with the full mediainfo command if available
	if len(output) == 0 || strings.TrimSpace(string(output)) == "{}" {
		// Try to find mediainfo
		mediainfoPath, err := exec.LookPath("mediainfo")
		if err == nil {
			// Run mediainfo command
			cmd = exec.Command(mediainfoPath, "--Output=JSON", absPath)
			output, err = cmd.CombinedOutput()
			if err != nil {
				return nil, fmt.Errorf("error running mediainfo: %w", err)
			}
		}
	}

	// Parse the output
	containerInfo := &ContainerInfo{
		VideoStreams:    make([]VideoStream, 0),
		AudioStreams:    make([]AudioStream, 0),
		SubtitleStreams: make([]SubtitleStream, 0),
	}

	// Set basic general info
	containerInfo.General.CompleteName = absPath

	// Check if we have JSON output
	if len(output) > 0 && output[0] == '{' {
		return p.parseJSONOutput(output, containerInfo)
	}

	// If not JSON, process as text output (legacy format or mediainfo)
	return p.parseTextOutput(string(output), containerInfo)
}

// parseJSONOutput parses JSON output from ffprobe and fills the ContainerInfo structure
func (p *Prober) parseJSONOutput(output []byte, containerInfo *ContainerInfo) (*ContainerInfo, error) {
	// Parse JSON output
	var jsonData map[string]interface{}
	err := json.Unmarshal(output, &jsonData)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON output: %w", err)
	}

	// Process format information (general info)
	if format, ok := jsonData["format"].(map[string]interface{}); ok {
		p.processJSONFormat(format, &containerInfo.General)
	}

	// Process streams
	if streams, ok := jsonData["streams"].([]interface{}); ok {
		for _, stream := range streams {
			streamMap, ok := stream.(map[string]interface{})
			if !ok {
				continue
			}

			// Determine stream type
			codecType, _ := streamMap["codec_type"].(string)
			switch codecType {
			case "video":
				var videoStream VideoStream
				p.processJSONVideoStream(streamMap, &videoStream)
				containerInfo.VideoStreams = append(containerInfo.VideoStreams, videoStream)
			case "audio":
				var audioStream AudioStream
				p.processJSONAudioStream(streamMap, &audioStream)
				containerInfo.AudioStreams = append(containerInfo.AudioStreams, audioStream)
			case "subtitle":
				var subtitleStream SubtitleStream
				p.processJSONSubtitleStream(streamMap, &subtitleStream)
				containerInfo.SubtitleStreams = append(containerInfo.SubtitleStreams, subtitleStream)
			}
		}
	}

	// Calculate video bit rates if they are not directly available
	p.calculateMissingBitRates(containerInfo)

	return containerInfo, nil
}

// parseTextOutput parses text output from mediainfo or other tools and fills the ContainerInfo structure
func (p *Prober) parseTextOutput(output string, containerInfo *ContainerInfo) (*ContainerInfo, error) {
	// Split the output into sections based on empty lines
	sections := regexp.MustCompile(`\n\s*\n`).Split(output, -1)

	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}

		// Parse the section title from the first line
		lines := strings.Split(section, "\n")
		if len(lines) == 0 {
			continue
		}

		titleLine := lines[0]
		sectionRegex := regexp.MustCompile(`^(General|Video|Audio|Text)( #\d+)?$`)
		sectionMatch := sectionRegex.FindStringSubmatch(titleLine)

		if sectionMatch == nil {
			continue
		}

		sectionType := sectionMatch[1]

		// Process the section based on its type
		switch sectionType {
		case "General":
			p.processGeneralInfo(section, &containerInfo.General)
		case "Video":
			videoStream := VideoStream{}
			p.processVideoStream(section, &videoStream)
			containerInfo.VideoStreams = append(containerInfo.VideoStreams, videoStream)
		case "Audio":
			audioStream := AudioStream{}
			p.processAudioStream(section, &audioStream)
			containerInfo.AudioStreams = append(containerInfo.AudioStreams, audioStream)
		case "Text":
			subtitleStream := SubtitleStream{}
			p.processSubtitleStream(section, &subtitleStream)
			containerInfo.SubtitleStreams = append(containerInfo.SubtitleStreams, subtitleStream)
		}
	}

	// Calculate video bit rates if they are not directly available
	p.calculateMissingBitRates(containerInfo)

	return containerInfo, nil
}

// calculateMissingBitRates calculates video bit rates when they are not directly available
func (p *Prober) calculateMissingBitRates(containerInfo *ContainerInfo) {
	// If we have overall bit rate and no video bit rates, estimate the video bit rates
	if containerInfo.General.OverallBitRate > 0 {
		// Calculate total audio bit rate
		var totalAudioBitRate int64
		for i := range containerInfo.AudioStreams {
			totalAudioBitRate += containerInfo.AudioStreams[i].BitRate
		}

		// Estimate video bit rates for video streams with no bit rate
		if len(containerInfo.VideoStreams) > 0 {
			// Calculate remaining bit rate after audio
			remainingBitRate := containerInfo.General.OverallBitRate - totalAudioBitRate

			// Count video streams with no bit rate
			videoStreamsWithNoBitRate := 0
			for i := range containerInfo.VideoStreams {
				if containerInfo.VideoStreams[i].BitRate == 0 {
					videoStreamsWithNoBitRate++
				}
			}

			// If we have video streams with no bit rate, distribute the remaining bit rate
			if videoStreamsWithNoBitRate > 0 {
				bitRatePerStream := remainingBitRate / int64(videoStreamsWithNoBitRate)
				for i := range containerInfo.VideoStreams {
					if containerInfo.VideoStreams[i].BitRate == 0 {
						containerInfo.VideoStreams[i].BitRate = bitRatePerStream
					}
				}
			}
		}
	}
}

// GetVideoInfo returns information about a video file
func (p *Prober) GetVideoInfo(filename string) (*VideoInfo, error) {
	// Get container info first
	containerInfo, err := p.GetExtendedContainerInfo(filename)
	if err != nil {
		return nil, fmt.Errorf("error getting container info: %w", err)
	}

	// Create a new VideoInfo from the container info
	videoInfo := p.createVideoInfoFromContainer(containerInfo)

	// Find the main video stream
	p.findAndProcessMainVideo(containerInfo, videoInfo)

	// Process streams
	p.processAudioStreams(containerInfo, videoInfo)
	p.processSubtitleStreams(containerInfo, videoInfo)

	return videoInfo, nil
}

// createVideoInfoFromContainer creates a new VideoInfo struct from ContainerInfo
func (p *Prober) createVideoInfoFromContainer(containerInfo *ContainerInfo) *VideoInfo {
	return &VideoInfo{
		FileName:     containerInfo.General.CompleteName,
		Format:       containerInfo.General.Format,
		Duration:     containerInfo.General.Duration,
		FileSizeMB:   float64(containerInfo.General.FileSize) / 1024 / 1024,
		BitRate:      containerInfo.General.OverallBitRate,
		VideoStreams: make([]VideoStream, 0),
		AudioStreams: make([]AudioStream, 0),
	}
}

// findAndProcessMainVideo finds the main video stream and processes it
func (p *Prober) findAndProcessMainVideo(containerInfo *ContainerInfo, videoInfo *VideoInfo) {
	if len(containerInfo.VideoStreams) > 0 {
		// Use the first video stream as the main video
		mainVideoStream := containerInfo.VideoStreams[0]

		// Set main video properties
		videoInfo.VideoFormat = mainVideoStream.Format
		videoInfo.Width = mainVideoStream.Width
		videoInfo.Height = mainVideoStream.Height
		videoInfo.AspectRatio = mainVideoStream.DisplayAspectRatio
		videoInfo.FrameRate = mainVideoStream.FrameRate

		// Add all video streams
		videoInfo.VideoStreams = containerInfo.VideoStreams
	}
}

// processAudioStreams processes audio streams from container info
func (p *Prober) processAudioStreams(containerInfo *ContainerInfo, videoInfo *VideoInfo) {
	for _, audioStream := range containerInfo.AudioStreams {
		videoInfo.AudioStreams = append(videoInfo.AudioStreams, audioStream)

		// Also add to simplified audio tracks list
		audioTrack := AudioTrack{
			Index:    audioStream.ID,
			Format:   audioStream.Format,
			Language: audioStream.Language,
			Channels: audioStream.Channels,
			Default:  audioStream.Default,
		}
		videoInfo.AudioTracks = append(videoInfo.AudioTracks, audioTrack)
	}
}

// processSubtitleStreams processes subtitle streams from container info
func (p *Prober) processSubtitleStreams(containerInfo *ContainerInfo, videoInfo *VideoInfo) {
	for _, subtitleStream := range containerInfo.SubtitleStreams {
		// Add to simplified subtitle tracks list
		subtitleTrack := SubtitleTrack{
			Index:    subtitleStream.ID,
			Format:   subtitleStream.Format,
			Language: subtitleStream.Language,
			Default:  subtitleStream.Default,
		}
		videoInfo.SubtitleTracks = append(videoInfo.SubtitleTracks, subtitleTrack)
	}
}

// Public functions (alphabetical)

// NewProber creates a new Prober instance
func NewProber(ffmpegInfo *FFmpegInfo) (*Prober, error) {
	if ffmpegInfo == nil || !ffmpegInfo.Installed {
		return nil, fmt.Errorf("FFmpeg is not installed")
	}

	// Replace ffmpeg with ffprobe in the path
	ffprobePath := strings.Replace(ffmpegInfo.Path, "ffmpeg", "ffprobe", 1)

	return &Prober{
		FFprobePath: ffprobePath,
	}, nil
}

// Type methods (alphabetical)

// String returns a string representation of VideoInfo
func (v *VideoInfo) String() string {
	var parts []string

	if v.VideoFormat != "" {
		parts = append(parts, fmt.Sprintf("VideoFormat: %s", v.VideoFormat))
	}

	if v.Width > 0 && v.Height > 0 {
		parts = append(parts, fmt.Sprintf("Resolution: %dx%d", v.Width, v.Height))
	}

	if v.FrameRate > 0 {
		parts = append(parts, fmt.Sprintf("FPS: %.3f", v.FrameRate))
	}

	if v.Duration > 0 {
		parts = append(parts, fmt.Sprintf("Duration: %fs", v.Duration))
	}

	return strings.Join(parts, ", ")
}

// processVideoStreamKey processes a single key-value pair for video stream
func (p *Prober) processVideoStreamKey(key, value string, videoStream *VideoStream) {
	// Group processing into categories
	switch {
	case p.isBasicVideoMetadata(key):
		p.processBasicVideoMetadata(key, value, videoStream)
	case p.isFormatVideoMetadata(key):
		p.processFormatVideoMetadata(key, value, videoStream)
	case p.isCodecVideoMetadata(key):
		p.processCodecVideoMetadata(key, value, videoStream)
	case p.isDimensionVideoMetadata(key):
		p.processDimensionVideoMetadata(key, value, videoStream)
	case p.isTimingVideoMetadata(key):
		p.processTimingVideoMetadata(key, value, videoStream)
	case p.isQualityVideoMetadata(key):
		p.processQualityVideoMetadata(key, value, videoStream)
	case p.isFlagVideoMetadata(key):
		p.processFlagVideoMetadata(key, value, videoStream)
	}
}

// isBasicVideoMetadata checks if key is for basic video metadata
func (p *Prober) isBasicVideoMetadata(key string) bool {
	return key == "ID" || key == "Language" || key == "Title"
}

// processBasicVideoMetadata handles basic metadata fields for video streams
func (p *Prober) processBasicVideoMetadata(key, value string, videoStream *VideoStream) {
	switch key {
	case "ID":
		videoStream.ID = value
	case "Language":
		videoStream.Language = value
	case "Title":
		videoStream.Title = value
	}
}

// isFormatVideoMetadata checks if key is for format-related video metadata
func (p *Prober) isFormatVideoMetadata(key string) bool {
	return key == "Format" || key == "Format/Info" || key == "Format profile" ||
		key == "Format settings" || key == "Format settings, CABAC" ||
		key == "Format settings, Reference frames"
}

// processFormatVideoMetadata handles format-related metadata fields for video streams
func (p *Prober) processFormatVideoMetadata(key, value string, videoStream *VideoStream) {
	switch key {
	case "Format":
		videoStream.Format = value
	case "Format/Info":
		videoStream.FormatInfo = value
	case "Format profile":
		videoStream.FormatProfile = value
	case "Format settings":
		videoStream.FormatSettings = value
	case "Format settings, CABAC":
		videoStream.FormatSettingsCABAC = value
	case "Format settings, Reference frames":
		refFrames, err := strconv.Atoi(value)
		if err == nil {
			videoStream.FormatSettingsRefFrames = refFrames
		}
	}
}

// isCodecVideoMetadata checks if key is for codec-related video metadata
func (p *Prober) isCodecVideoMetadata(key string) bool {
	return key == "Codec ID" || key == "Codec ID/Info" || key == "Codec ID/Hint"
}

// processCodecVideoMetadata handles codec-related metadata fields for video streams
func (p *Prober) processCodecVideoMetadata(key, value string, videoStream *VideoStream) {
	switch key {
	case "Codec ID":
		videoStream.CodecID = value
	case "Codec ID/Info":
		videoStream.CodecIDInfo = value
	case "Codec ID/Hint":
		videoStream.CodecIDHint = value
	}
}

// isDimensionVideoMetadata checks if key is for dimension-related video metadata
func (p *Prober) isDimensionVideoMetadata(key string) bool {
	return key == "Width" || key == "Height" || key == "Display aspect ratio"
}

// processDimensionVideoMetadata handles dimension-related metadata fields for video streams
func (p *Prober) processDimensionVideoMetadata(key, value string, videoStream *VideoStream) {
	switch key {
	case "Width":
		width, err := strconv.Atoi(value)
		if err == nil {
			videoStream.Width = width
		}
	case "Height":
		height, err := strconv.Atoi(value)
		if err == nil {
			videoStream.Height = height
		}
	case "Display aspect ratio":
		// Parse display aspect ratio
		if strings.Contains(value, ":") {
			parts := strings.Split(value, ":")
			if len(parts) == 2 {
				if width, err := strconv.ParseFloat(parts[0], 64); err == nil {
					if height, err := strconv.ParseFloat(parts[1], 64); err == nil && height > 0 {
						videoStream.DisplayAspectRatio = width / height
					}
				}
			}
		} else {
			if ratio, err := strconv.ParseFloat(value, 64); err == nil {
				videoStream.DisplayAspectRatio = ratio
			}
		}
		videoStream.AspectRatio = value
	}
}

// isTimingVideoMetadata checks if key is for timing-related video metadata
func (p *Prober) isTimingVideoMetadata(key string) bool {
	return key == "Duration" || key == "Frame rate"
}

// processTimingVideoMetadata handles timing-related metadata fields for video streams
func (p *Prober) processTimingVideoMetadata(key, value string, videoStream *VideoStream) {
	switch key {
	case "Duration":
		p.parseDuration(value, &videoStream.Duration)
	case "Frame rate":
		if frameRate, err := strconv.ParseFloat(value, 64); err == nil {
			videoStream.FrameRate = frameRate
		}
	}
}

// isQualityVideoMetadata checks if key is for quality-related video metadata
func (p *Prober) isQualityVideoMetadata(key string) bool {
	return key == "Bit rate" || key == "Bit depth" || key == "Color space" ||
		key == "Chroma subsampling" || key == "Compression mode" ||
		key == "Bits/(Pixel*Frame)"
}

// processQualityVideoMetadata handles quality-related metadata fields for video streams
func (p *Prober) processQualityVideoMetadata(key, value string, videoStream *VideoStream) {
	switch key {
	case "Bit rate":
		p.parseBitRate(value, &videoStream.BitRate)
	case "Bit depth":
		bitDepth, err := strconv.Atoi(value)
		if err == nil {
			videoStream.BitDepth = bitDepth
		}
	case "Color space":
		videoStream.ColorSpace = value
	case "Chroma subsampling":
		videoStream.ChromaSubsampling = value
	case "Compression mode":
		videoStream.CompressionMode = value
	case "Bits/(Pixel*Frame)":
		if bpf, err := strconv.ParseFloat(value, 64); err == nil {
			videoStream.BitsPerPixel = bpf
		}
	}
}

// isFlagVideoMetadata checks if key is for flag-related video metadata
func (p *Prober) isFlagVideoMetadata(key string) bool {
	return key == "Default" || key == "Forced"
}

// processFlagVideoMetadata handles flag-related metadata fields for video streams
func (p *Prober) processFlagVideoMetadata(key, value string, videoStream *VideoStream) {
	switch key {
	case "Default":
		videoStream.Default = (value == "Yes")
	case "Forced":
		videoStream.Forced = (value == "Yes")
	}
}

// parseDuration parses a duration string into seconds
func (p *Prober) parseDuration(duration string, result *float64) {
	// Format is expected to be like: 1h 42mn or like: 1h 42 min, etc.
	// Try to parse to seconds
	seconds := 0.0
	durationParts := strings.Fields(duration)

	for _, part := range durationParts {
		if strings.Contains(part, "h") {
			hours, err := strconv.ParseFloat(strings.TrimSuffix(part, "h"), 64)
			if err == nil {
				seconds += hours * 3600
			}
		} else if strings.Contains(part, "mn") || strings.Contains(part, "min") {
			minutes, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSuffix(part, "mn"), "min"), 64)
			if err == nil {
				seconds += minutes * 60
			}
		} else if strings.Contains(part, "s") && !strings.Contains(part, "ms") {
			secs, err := strconv.ParseFloat(strings.TrimSuffix(part, "s"), 64)
			if err == nil {
				seconds += secs
			}
		} else if strings.Contains(part, "ms") {
			ms, err := strconv.ParseFloat(strings.TrimSuffix(part, "ms"), 64)
			if err == nil {
				seconds += ms / 1000
			}
		}
	}

	*result = seconds
}

// parseBitRate parses a bit rate string into bits per second
func (p *Prober) parseBitRate(bitrateStr string, result *int64) {
	// Remove spaces in numeric part, e.g., "5 000" -> "5000"
	bitrateStr = strings.ReplaceAll(bitrateStr, " ", "")
	parts := strings.Fields(bitrateStr)

	if len(parts) > 0 {
		valueStr := parts[0]
		// Remove any non-numeric characters except decimal point
		for i, c := range valueStr {
			if !unicode.IsDigit(c) && c != '.' {
				valueStr = valueStr[:i]
				break
			}
		}

		bitrate, err := strconv.ParseInt(valueStr, 10, 64)
		if err == nil {
			// Convert to bits per second based on unit
			if len(parts) > 1 {
				unit := strings.ToLower(parts[1])
				if strings.HasPrefix(unit, "kb") {
					bitrate *= 1000
				} else if strings.HasPrefix(unit, "mb") {
					bitrate *= 1000000
				} else if strings.HasPrefix(unit, "gb") {
					bitrate *= 1000000000
				}
			}
			*result = bitrate
		}
	}
}
