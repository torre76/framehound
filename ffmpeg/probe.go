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
)

// Private methods (alphabetical)

// processAudioStream processes a key-value pair for an AudioStream
func (p *Prober) processAudioStream(stream *AudioStream, key, value string) {
	switch key {
	case "ID":
		stream.ID = value
	case "Format":
		stream.Format = value
	case "Format/Info":
		stream.FormatInfo = value
	case "Commercial name":
		stream.CommercialName = value
	case "Codec ID":
		stream.CodecID = value
	case "Duration":
		// Parse duration (e.g., "28 min 18 s")
		// Simplified approach - in a real implementation, you would parse more complex durations
		durationStr := strings.ReplaceAll(value, " ", "")
		minutes := 0
		seconds := 0

		minIndex := strings.Index(durationStr, "min")
		if minIndex > 0 {
			minStr := durationStr[:minIndex]
			minutes, _ = strconv.Atoi(minStr)
			durationStr = durationStr[minIndex+3:]
		}

		secIndex := strings.Index(durationStr, "s")
		if secIndex > 0 {
			secStr := durationStr[:secIndex]
			seconds, _ = strconv.Atoi(secStr)
		}

		stream.Duration = float64(minutes*60 + seconds)
	case "Bit rate mode":
		stream.BitRateMode = value
	case "Bit rate":
		// Parse bit rate (e.g., "768 kb/s")
		parts := strings.Split(value, " ")
		if len(parts) >= 1 {
			bitRate, err := strconv.Atoi(parts[0])
			if err == nil {
				// Convert to bits per second based on unit
				if len(parts) > 1 {
					unit := strings.ToLower(parts[1])
					switch {
					case strings.HasPrefix(unit, "kb"):
						bitRate *= 1000
					case strings.HasPrefix(unit, "mb"):
						bitRate *= 1000 * 1000
					case strings.HasPrefix(unit, "gb"):
						bitRate *= 1000 * 1000 * 1000
					}
				}
				stream.BitRate = int64(bitRate)
			}
		}
	case "Channel(s)":
		// Parse channels (e.g., "6 channels")
		parts := strings.Split(value, " ")
		if len(parts) >= 1 {
			channels, err := strconv.Atoi(parts[0])
			if err == nil {
				stream.Channels = channels
			}
		}
	case "Channel layout":
		stream.ChannelLayout = value
	case "Sampling rate":
		// Parse sampling rate (e.g., "48.0 kHz")
		parts := strings.Split(value, " ")
		if len(parts) >= 1 {
			rate, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				// Convert to Hz based on unit
				if len(parts) > 1 {
					unit := strings.ToLower(parts[1])
					if strings.HasPrefix(unit, "k") {
						rate *= 1000
					}
				}
				stream.SamplingRate = rate
			}
		}
	case "Frame rate":
		// Parse frame rate (e.g., "31.250 FPS (1536 SPF)")
		parts := strings.Split(value, " ")
		if len(parts) >= 1 {
			frameRate, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				stream.FrameRate = frameRate
			}
		}
	case "Compression mode":
		stream.CompressionMode = value
	case "Stream size":
		// Parse stream size (e.g., "155 MiB (7%)")
		parts := strings.Split(value, " ")
		if len(parts) >= 2 {
			size, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				// Convert to bytes based on unit
				unit := strings.ToLower(parts[1])
				switch {
				case strings.HasPrefix(unit, "kb"):
					size *= 1024
				case strings.HasPrefix(unit, "mb"):
					size *= 1024 * 1024
				case strings.HasPrefix(unit, "gb"):
					size *= 1024 * 1024 * 1024
				case strings.HasPrefix(unit, "tb"):
					size *= 1024 * 1024 * 1024 * 1024
				}
				stream.StreamSize = int64(size)
			}
		}
	case "Title":
		stream.Title = value
	case "Language":
		stream.Language = value
	case "Default":
		stream.Default = (value == "Yes")
	case "Forced":
		stream.Forced = (value == "Yes")
	}
}

// processGeneralInfo processes a key-value pair for GeneralInfo
func (p *Prober) processGeneralInfo(info *GeneralInfo, key, value string) {
	switch key {
	case "Unique ID":
		info.UniqueID = value
	case "Complete name":
		info.CompleteName = value
	case "Format":
		info.Format = value
	case "Format version":
		info.FormatVersion = value
	case "File size":
		// Parse file size (e.g., "2.19 GiB")
		parts := strings.Split(value, " ")
		if len(parts) >= 1 {
			size, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				// Convert to bytes based on unit
				if len(parts) > 1 {
					unit := strings.ToLower(parts[1])
					switch {
					case strings.HasPrefix(unit, "k"):
						size *= 1024
					case strings.HasPrefix(unit, "m"):
						size *= 1024 * 1024
					case strings.HasPrefix(unit, "g"):
						size *= 1024 * 1024 * 1024
					case strings.HasPrefix(unit, "t"):
						size *= 1024 * 1024 * 1024 * 1024
					}
				}
				info.FileSize = int64(size)
			}
		}
	case "Duration":
		// Parse duration (e.g., "28 min 18 s")
		// Simplified approach - in a real implementation, you would parse more complex durations
		durationStr := strings.ReplaceAll(value, " ", "")
		minutes := 0
		seconds := 0

		minIndex := strings.Index(durationStr, "min")
		if minIndex > 0 {
			minStr := durationStr[:minIndex]
			minutes, _ = strconv.Atoi(minStr)
			durationStr = durationStr[minIndex+3:]
		}

		secIndex := strings.Index(durationStr, "s")
		if secIndex > 0 {
			secStr := durationStr[:secIndex]
			seconds, _ = strconv.Atoi(secStr)
		}

		info.Duration = float64(minutes*60 + seconds)
	case "Overall bit rate":
		// Parse bit rate (e.g., "11.1 Mb/s")
		parts := strings.Split(value, " ")
		if len(parts) >= 1 {
			bitRate, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				// Convert to bits per second based on unit
				if len(parts) > 1 {
					unit := strings.ToLower(parts[1])
					switch {
					case strings.HasPrefix(unit, "kb"):
						bitRate *= 1000
					case strings.HasPrefix(unit, "mb"):
						bitRate *= 1000 * 1000
					case strings.HasPrefix(unit, "gb"):
						bitRate *= 1000 * 1000 * 1000
					}
				}
				info.OverallBitRate = int64(bitRate)
			}
		}
	case "Frame rate":
		// Parse frame rate (e.g., "23.976 FPS")
		parts := strings.Split(value, " ")
		if len(parts) >= 1 {
			frameRate, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				info.FrameRate = frameRate
			}
		}
	case "Encoded date":
		info.EncodedDate = value
	case "Writing application":
		info.WritingApplication = value
	case "Writing library":
		info.WritingLibrary = value
	}
}

// processJSONAudioStream processes JSON audio stream data
func (p *Prober) processJSONAudioStream(stream map[string]interface{}, audioStream *AudioStream) {
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

	// Extract channels
	if channels, ok := stream["channels"].(float64); ok {
		audioStream.Channels = int(channels)
	}

	// Extract channel layout
	if channelLayout, ok := stream["channel_layout"].(string); ok {
		audioStream.ChannelLayout = channelLayout
	}

	// Extract sample rate
	if sampleRateStr, ok := stream["sample_rate"].(string); ok {
		sampleRate, err := strconv.ParseFloat(sampleRateStr, 64)
		if err == nil {
			audioStream.SamplingRate = sampleRate
		}
	}

	// Extract bit rate
	if bitrateStr, ok := stream["bit_rate"].(string); ok {
		bitrate, err := strconv.ParseInt(bitrateStr, 10, 64)
		if err == nil {
			audioStream.BitRate = bitrate
		}
	}

	// Extract duration
	if durationStr, ok := stream["duration"].(string); ok {
		duration, err := strconv.ParseFloat(durationStr, 64)
		if err == nil {
			audioStream.Duration = duration
		}
	}

	// Extract tags if available
	if tags, ok := stream["tags"].(map[string]interface{}); ok {
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
		parts := strings.Split(dar, ":")
		if len(parts) == 2 {
			num, err1 := strconv.ParseFloat(parts[0], 64)
			den, err2 := strconv.ParseFloat(parts[1], 64)
			if err1 == nil && err2 == nil && den > 0 {
				videoStream.DisplayAspectRatio = num / den
			}
		}
	}

	// Extract bit rate
	if bitrateStr, ok := stream["bit_rate"].(string); ok {
		bitrate, err := strconv.ParseInt(bitrateStr, 10, 64)
		if err == nil {
			videoStream.BitRate = bitrate
		}
	}

	// Extract frame rate
	if fpsStr, ok := stream["r_frame_rate"].(string); ok {
		parts := strings.Split(fpsStr, "/")
		if len(parts) == 2 {
			num, err1 := strconv.ParseFloat(parts[0], 64)
			den, err2 := strconv.ParseFloat(parts[1], 64)
			if err1 == nil && err2 == nil && den > 0 {
				videoStream.FrameRate = num / den
			}
		}
	}

	// Extract duration
	if durationStr, ok := stream["duration"].(string); ok {
		duration, err := strconv.ParseFloat(durationStr, 64)
		if err == nil {
			videoStream.Duration = duration
		}
	}

	// Extract bit depth
	if profile, ok := stream["profile"].(string); ok {
		if strings.Contains(profile, "10") {
			videoStream.BitDepth = 10
		} else if strings.Contains(profile, "12") {
			videoStream.BitDepth = 12
		} else {
			videoStream.BitDepth = 8
		}
	}

	// Extract color space
	if colorSpace, ok := stream["color_space"].(string); ok {
		videoStream.ColorSpace = colorSpace
	}

	// Extract chroma subsampling
	if chromaLocation, ok := stream["chroma_location"].(string); ok {
		videoStream.ChromaSubsampling = chromaLocation
	}

	// Extract tags if available
	if tags, ok := stream["tags"].(map[string]interface{}); ok {
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
}

// processSubtitleStream processes a key-value pair for a SubtitleStream
func (p *Prober) processSubtitleStream(stream *SubtitleStream, key, value string) {
	switch key {
	case "ID":
		stream.ID = value
	case "Format":
		stream.Format = value
	case "Codec ID":
		stream.CodecID = value
	case "Codec ID/Info":
		stream.CodecIDInfo = value
	case "Duration":
		// Parse duration (e.g., "10 min 10 s")
		// Simplified approach - in a real implementation, you would parse more complex durations
		durationStr := strings.ReplaceAll(value, " ", "")
		minutes := 0
		seconds := 0

		minIndex := strings.Index(durationStr, "min")
		if minIndex > 0 {
			minStr := durationStr[:minIndex]
			minutes, _ = strconv.Atoi(minStr)
			durationStr = durationStr[minIndex+3:]
		}

		secIndex := strings.Index(durationStr, "s")
		if secIndex > 0 {
			secStr := durationStr[:secIndex]
			seconds, _ = strconv.Atoi(secStr)
		}

		stream.Duration = float64(minutes*60 + seconds)
	case "Bit rate":
		// Parse bit rate (e.g., "3 b/s")
		parts := strings.Split(value, " ")
		if len(parts) >= 1 {
			bitRate, err := strconv.Atoi(parts[0])
			if err == nil {
				stream.BitRate = int64(bitRate)
			}
		}
	case "Frame rate":
		// Parse frame rate (e.g., "0.013 FPS")
		parts := strings.Split(value, " ")
		if len(parts) >= 1 {
			frameRate, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				stream.FrameRate = frameRate
			}
		}
	case "Count of elements":
		// Parse count of elements (e.g., "8")
		count, err := strconv.Atoi(value)
		if err == nil {
			stream.CountOfElements = count
		}
	case "Stream size":
		// Parse stream size (e.g., "244 Bytes (0%)")
		parts := strings.Split(value, " ")
		if len(parts) >= 2 {
			size, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				// Convert to bytes based on unit
				unit := strings.ToLower(parts[1])
				switch {
				case strings.HasPrefix(unit, "kb"):
					size *= 1024
				case strings.HasPrefix(unit, "mb"):
					size *= 1024 * 1024
				case strings.HasPrefix(unit, "gb"):
					size *= 1024 * 1024 * 1024
				case strings.HasPrefix(unit, "tb"):
					size *= 1024 * 1024 * 1024 * 1024
				}
				stream.StreamSize = int64(size)
			}
		}
	case "Title":
		stream.Title = value
	case "Language":
		stream.Language = value
	case "Default":
		stream.Default = (value == "Yes")
	case "Forced":
		stream.Forced = (value == "Yes")
	}
}

// processVideoStream processes a key-value pair for a VideoStream
func (p *Prober) processVideoStream(stream *VideoStream, key, value string) {
	switch key {
	case "ID":
		stream.ID = value
	case "Format":
		stream.Format = value
	case "Format/Info":
		stream.FormatInfo = value
	case "Format profile":
		stream.FormatProfile = value
	case "Format settings":
		stream.FormatSettings = value
	case "Codec ID":
		stream.CodecID = value
	case "Duration":
		// Parse duration (e.g., "28 min 18 s")
		// Simplified approach - in a real implementation, you would parse more complex durations
		durationStr := strings.ReplaceAll(value, " ", "")
		minutes := 0
		seconds := 0

		minIndex := strings.Index(durationStr, "min")
		if minIndex > 0 {
			minStr := durationStr[:minIndex]
			minutes, _ = strconv.Atoi(minStr)
			durationStr = durationStr[minIndex+3:]
		}

		secIndex := strings.Index(durationStr, "s")
		if secIndex > 0 {
			secStr := durationStr[:secIndex]
			seconds, _ = strconv.Atoi(secStr)
		}

		stream.Duration = float64(minutes*60 + seconds)
	case "Bit rate":
		// Parse bit rate (e.g., "9 918 kb/s")
		value = strings.ReplaceAll(value, " ", "")
		parts := strings.Split(value, "kb/s")
		if len(parts) >= 1 {
			bitRate, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				stream.BitRate = int64(bitRate * 1000) // Convert to bits per second
			}
		}
	case "Width":
		// Parse width (e.g., "1 920 pixels")
		value = strings.ReplaceAll(value, " ", "")
		parts := strings.Split(value, "pixels")
		if len(parts) >= 1 {
			width, err := strconv.Atoi(parts[0])
			if err == nil {
				stream.Width = width
			}
		}
	case "Height":
		// Parse height (e.g., "960 pixels")
		value = strings.ReplaceAll(value, " ", "")
		parts := strings.Split(value, "pixels")
		if len(parts) >= 1 {
			height, err := strconv.Atoi(parts[0])
			if err == nil {
				stream.Height = height
			}
		}
	case "Display aspect ratio":
		// Parse aspect ratio (e.g., "2.000")
		ratio, err := strconv.ParseFloat(value, 64)
		if err == nil {
			stream.DisplayAspectRatio = ratio
		}
	case "Frame rate mode":
		stream.FrameRateMode = value
	case "Frame rate":
		// Parse frame rate (e.g., "23.976 (24000/1001) FPS")
		parts := strings.Split(value, " ")
		if len(parts) >= 1 {
			frameRate, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				stream.FrameRate = frameRate
			}
		}
	case "Color space":
		stream.ColorSpace = value
	case "Chroma subsampling":
		stream.ChromaSubsampling = value
	case "Bit depth":
		// Parse bit depth (e.g., "8 bits")
		parts := strings.Split(value, " ")
		if len(parts) >= 1 {
			bitDepth, err := strconv.Atoi(parts[0])
			if err == nil {
				stream.BitDepth = bitDepth
			}
		}
	case "Scan type":
		stream.ScanType = value
	case "Stream size":
		// Parse stream size (e.g., "1.96 GiB (90%)")
		parts := strings.Split(value, " ")
		if len(parts) >= 2 {
			size, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				// Convert to bytes based on unit
				unit := strings.ToLower(parts[1])
				switch {
				case strings.HasPrefix(unit, "kb"):
					size *= 1024
				case strings.HasPrefix(unit, "mb"):
					size *= 1024 * 1024
				case strings.HasPrefix(unit, "gb"):
					size *= 1024 * 1024 * 1024
				case strings.HasPrefix(unit, "tb"):
					size *= 1024 * 1024 * 1024 * 1024
				}
				stream.StreamSize = int64(size)
			}
		}
	case "Title":
		stream.Title = value
	case "Language":
		stream.Language = value
	case "Default":
		stream.Default = (value == "Yes")
	case "Forced":
		stream.Forced = (value == "Yes")
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
		// Parse JSON output
		var jsonData map[string]interface{}
		err = json.Unmarshal(output, &jsonData)
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

	// If not JSON, process as text output (legacy format or mediainfo)
	// Process the output line by line to extract information
	lines := strings.Split(string(output), "\n")

	// Variables to track the current section
	var currentSection string
	var currentStreamIndex int

	// Regular expressions to match section headers and key-value pairs
	sectionRegex := regexp.MustCompile(`^(General|Video|Audio|Text)( #\d+)?$`)
	kvRegex := regexp.MustCompile(`^([^:]+)\s*:\s*(.+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if this is a section header
		sectionMatch := sectionRegex.FindStringSubmatch(line)
		if sectionMatch != nil {
			currentSection = sectionMatch[1]
			currentStreamIndex = 0
			if len(sectionMatch) > 2 && sectionMatch[2] != "" {
				// Extract stream index if present
				indexStr := strings.TrimPrefix(sectionMatch[2], " #")
				index, err := strconv.Atoi(indexStr)
				if err == nil {
					currentStreamIndex = index
				}
			}
			continue
		}

		// Parse key-value pair
		kvMatch := kvRegex.FindStringSubmatch(line)
		if kvMatch == nil {
			continue
		}

		key := strings.TrimSpace(kvMatch[1])
		value := strings.TrimSpace(kvMatch[2])

		// Process based on current section
		switch currentSection {
		case "General":
			p.processGeneralInfo(&containerInfo.General, key, value)
		case "Video":
			// Ensure we have enough video streams
			for len(containerInfo.VideoStreams) <= currentStreamIndex {
				containerInfo.VideoStreams = append(containerInfo.VideoStreams, VideoStream{})
			}
			p.processVideoStream(&containerInfo.VideoStreams[currentStreamIndex], key, value)
		case "Audio":
			// Ensure we have enough audio streams
			for len(containerInfo.AudioStreams) <= currentStreamIndex {
				containerInfo.AudioStreams = append(containerInfo.AudioStreams, AudioStream{})
			}
			p.processAudioStream(&containerInfo.AudioStreams[currentStreamIndex], key, value)
		case "Text":
			// Ensure we have enough subtitle streams
			for len(containerInfo.SubtitleStreams) <= currentStreamIndex {
				containerInfo.SubtitleStreams = append(containerInfo.SubtitleStreams, SubtitleStream{})
			}
			p.processSubtitleStream(&containerInfo.SubtitleStreams[currentStreamIndex], key, value)
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

// GetVideoInfo runs ffprobe on the given file and returns structured video information
func (p *Prober) GetVideoInfo(filePath string) (*VideoInfo, error) {
	// Run ffprobe command to get video information
	cmd := exec.Command(p.FFprobePath, "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=codec_name,width,height,r_frame_rate,duration",
		"-of", "default=noprint_wrappers=1", filePath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error running ffprobe: %w", err)
	}

	// Parse the output
	info := &VideoInfo{FilePath: filePath}

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "codec_name":
			info.Codec = value
		case "width":
			width, err := strconv.Atoi(value)
			if err == nil {
				info.Width = width
			}
		case "height":
			height, err := strconv.Atoi(value)
			if err == nil {
				info.Height = height
			}
		case "r_frame_rate":
			// Frame rate is in the format "num/den"
			frParts := strings.Split(value, "/")
			if len(frParts) == 2 {
				num, err1 := strconv.ParseFloat(frParts[0], 64)
				den, err2 := strconv.ParseFloat(frParts[1], 64)
				if err1 == nil && err2 == nil && den > 0 {
					info.FrameRate = num / den
				}
			}
		case "duration":
			duration, err := strconv.ParseFloat(value, 64)
			if err == nil {
				info.Duration = duration
			}
		}
	}

	return info, nil
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

	if v.Codec != "" {
		parts = append(parts, fmt.Sprintf("Codec: %s", v.Codec))
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
