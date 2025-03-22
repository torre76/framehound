// Package main provides the entry point for the framehound application.
// It analyzes video files to extract frame-by-frame bitrate information and
// provides comprehensive container information analysis.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/torre76/framehound/ffmpeg"
	"github.com/urfave/cli/v2"
)

// Private constants (alphabetical)
// None currently defined

// Public constants (alphabetical)
// None currently defined

// Private variables (alphabetical)
// None currently defined

// Public variables (alphabetical)

// BuildDate contains the date when the binary was built.
// This value is set during build using ldflags.
var BuildDate = "unknown"

// Commit contains the git commit hash that the binary was built from.
// This value is set during build using ldflags.
var Commit = "unknown"

// Version contains the current version of the application.
// This value can be overridden during build using ldflags:
// go build -ldflags="-X 'github.com/torre76/framehound.Version=v1.0.0'"
var Version = "Development Version"

// Private functions (alphabetical)

// formatWithThousandSeparators formats an integer with thousand separators.
// It takes an int64 value and returns a string with commas separating thousands.
func formatWithThousandSeparators(n int64) string {
	in := strconv.FormatInt(n, 10)
	numOfDigits := len(in)
	if n < 0 {
		numOfDigits-- // First character is the - sign (not a digit)
	}
	numOfCommas := (numOfDigits - 1) / 3

	if numOfCommas == 0 {
		return in
	}

	var out string
	if n < 0 {
		in = in[1:] // Remove the - sign
		out = "-"
	}

	offset := len(in) % 3
	if offset > 0 {
		out += in[:offset] + ","
	}

	for i := offset; i < len(in); i += 3 {
		end := i + 3
		if end > len(in) {
			end = len(in)
		}
		out += in[i:end]
		if end < len(in) {
			out += ","
		}
	}
	return out
}

// printContainerInfo prints detailed information about the media container.
// It displays general information, video streams, audio streams, and subtitle streams
// using consistent formatting and emoji indicators.
func printContainerInfo(info *ffmpeg.ContainerInfo) {
	summaryStyle := color.New(color.Bold, color.FgCyan)
	valueStyle := color.New(color.Bold)
	regularStyle := color.New(color.Reset)

	summaryStyle.Println("\n📊 Container Information:")
	regularStyle.Println("---------------------")

	// Get the filename from the Tags map
	regularStyle.Printf("📄 File: ")
	fileName := ""
	if info.General.Tags != nil {
		if path, ok := info.General.Tags["file_path"]; ok {
			fileName = path
		}
	}
	valueStyle.Printf("%s\n", fileName)

	regularStyle.Printf("📦 Format: ")
	valueStyle.Printf("%s\n", info.General.Format)

	regularStyle.Printf("💾 Size: ")
	valueStyle.Printf("%s\n", info.General.Size)

	regularStyle.Printf("⏱️ Duration: ")
	valueStyle.Printf("%.3f seconds\n", info.General.DurationF)

	regularStyle.Printf("⚡ Overall bitrate: ")
	// Parse bit rate from string
	bitRate := int64(0)
	if info.General.BitRate != "" {
		parts := strings.Fields(info.General.BitRate)
		if len(parts) >= 1 {
			// Handle formats like "5 000" by removing spaces
			valueStr := strings.ReplaceAll(parts[0], " ", "")
			parsedBitRate, err := strconv.ParseInt(valueStr, 10, 64)
			if err == nil {
				// Convert to bits per second based on unit
				if len(parts) > 1 {
					unit := strings.ToLower(parts[1])
					if strings.HasPrefix(unit, "kb") {
						parsedBitRate *= 1000
					} else if strings.HasPrefix(unit, "mb") {
						parsedBitRate *= 1000000
					} else if strings.HasPrefix(unit, "gb") {
						parsedBitRate *= 1000000000
					}
				}
				bitRate = parsedBitRate
			}
		}
	}
	valueStyle.Printf("%.2f Kbps\n", float64(bitRate)/1000)

	// Frame rate is not directly available in the new GeneralInfo struct
	// We can calculate it from the first video stream if available
	frameRate := 0.0
	if len(info.VideoStreams) > 0 {
		frameRate = info.VideoStreams[0].FrameRate
	}
	regularStyle.Printf("🖼️ Frame rate: ")
	valueStyle.Printf("%.3f fps\n", frameRate)

	if len(info.VideoStreams) > 0 {
		summaryStyle.Println("\n🎬 Video Streams:")
		regularStyle.Println("-------------")
		for i, stream := range info.VideoStreams {
			regularStyle.Printf("Stream #%d:\n", i)
			regularStyle.Printf("  🎞️ Codec: ")
			valueStyle.Printf("%s (%s)\n", stream.Format, stream.FormatProfile)
			regularStyle.Printf("  📐 Resolution: ")
			valueStyle.Printf("%dx%d pixels\n", stream.Width, stream.Height)
			regularStyle.Printf("  📺 Display Aspect Ratio: ")
			valueStyle.Printf("%.3f\n", stream.DisplayAspectRatio)
			regularStyle.Printf("  🔍 Bit depth: ")
			valueStyle.Printf("%d bits\n", stream.BitDepth)
			regularStyle.Printf("  ⚡ Bit rate: ")
			valueStyle.Printf("%.2f Kbps\n", float64(stream.BitRate)/1000)
			regularStyle.Printf("  🖼️ Frame rate: ")
			valueStyle.Printf("%.3f fps\n", stream.FrameRate)
			regularStyle.Printf("  📲 Scan type: ")
			valueStyle.Printf("%s\n", stream.ScanType)
			regularStyle.Printf("  🎨 Color space: ")
			valueStyle.Printf("%s\n", stream.ColorSpace)
		}
	}

	if len(info.AudioStreams) > 0 {
		summaryStyle.Println("\n🔊 Audio Streams:")
		regularStyle.Println("-------------")
		for i, stream := range info.AudioStreams {
			regularStyle.Printf("Stream #%d:\n", i)
			regularStyle.Printf("  🎚️ Codec: ")
			valueStyle.Printf("%s\n", stream.Format)
			regularStyle.Printf("  🔈 Channels: ")
			valueStyle.Printf("%d (%s)\n", stream.Channels, stream.ChannelLayout)
			regularStyle.Printf("  📊 Sample rate: ")
			valueStyle.Printf("%d Hz\n", stream.SamplingRate)
			regularStyle.Printf("  ⚡ Bit rate: ")
			valueStyle.Printf("%.2f Kbps\n", float64(stream.BitRate)/1000)
			regularStyle.Printf("  🌐 Language: ")
			valueStyle.Printf("%s\n", stream.Language)
		}
	}

	if len(info.SubtitleStreams) > 0 {
		summaryStyle.Println("\n💬 Subtitle Streams:")
		regularStyle.Println("----------------")
		for i, stream := range info.SubtitleStreams {
			regularStyle.Printf("Stream #%d:\n", i)
			regularStyle.Printf("  📝 Codec: ")
			valueStyle.Printf("%s\n", stream.Format)
			regularStyle.Printf("  🌐 Language: ")
			valueStyle.Printf("%s\n", stream.Language)
			regularStyle.Printf("  📌 Title: ")
			valueStyle.Printf("%s\n", stream.Title)
		}
	}
}

// saveContainerInfo saves the container information to a JSON file in the specified directory.
// It returns an error if the directory cannot be created or the file cannot be written.
func saveContainerInfo(info *ffmpeg.ContainerInfo, outputDir string) error {
	// Create the output directory if it doesn't exist
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}

	// Determine the output filename based on the input filename
	fileName := "container_info.json"
	if info.General.Tags != nil {
		if path, ok := info.General.Tags["file_path"]; ok {
			baseName := filepath.Base(path)
			fileName = strings.TrimSuffix(baseName, filepath.Ext(baseName)) + "_info.json"
		}
	}
	outputPath := filepath.Join(outputDir, fileName)

	// Create a simplified representation for JSON output
	jsonOutput := map[string]interface{}{
		"filename": "",
		"format": map[string]interface{}{
			"name":        info.General.Format,
			"description": "", // No FormatVersion in new struct
			"size":        info.General.Size,
			"duration":    info.General.DurationF,
			"bitrate":     info.General.BitRate,
			"framerate":   0.0, // Will set from video stream if available
		},
		"video_streams":    []interface{}{},
		"audio_streams":    []interface{}{},
		"subtitle_streams": []interface{}{},
	}

	// Get filename from Tags
	if info.General.Tags != nil {
		if path, ok := info.General.Tags["file_path"]; ok {
			jsonOutput["filename"] = path
		}
	}

	// Get frame rate from first video stream if available
	if len(info.VideoStreams) > 0 {
		jsonOutput["format"].(map[string]interface{})["framerate"] = info.VideoStreams[0].FrameRate
	}

	// Process video streams
	videoStreams := []interface{}{}
	for _, stream := range info.VideoStreams {
		videoStream := map[string]interface{}{
			"codec":        stream.Format,
			"profile":      stream.FormatProfile,
			"width":        stream.Width,
			"height":       stream.Height,
			"aspect_ratio": stream.DisplayAspectRatio,
			"bit_depth":    stream.BitDepth,
			"bit_rate":     stream.BitRate,
			"frame_rate":   stream.FrameRate,
			"scan_type":    stream.ScanType,
			"color_space":  stream.ColorSpace,
		}
		videoStreams = append(videoStreams, videoStream)
	}
	jsonOutput["video_streams"] = videoStreams

	// Process audio streams
	audioStreams := []interface{}{}
	for _, stream := range info.AudioStreams {
		audioStream := map[string]interface{}{
			"codec":          stream.Format,
			"channels":       stream.Channels,
			"channel_layout": stream.ChannelLayout,
			"sample_rate":    stream.SamplingRate,
			"bit_rate":       stream.BitRate,
			"language":       stream.Language,
		}
		audioStreams = append(audioStreams, audioStream)
	}
	jsonOutput["audio_streams"] = audioStreams

	// Process subtitle streams
	subtitleStreams := []interface{}{}
	for _, stream := range info.SubtitleStreams {
		subtitleStream := map[string]interface{}{
			"codec":    stream.Format,
			"language": stream.Language,
			"title":    stream.Title,
		}
		subtitleStreams = append(subtitleStreams, subtitleStream)
	}
	jsonOutput["subtitle_streams"] = subtitleStreams

	// Add metadata
	jsonOutput["analysis_info"] = map[string]interface{}{
		"timestamp":  time.Now().Format(time.RFC3339),
		"version":    Version,
		"build_date": BuildDate,
	}

	// Marshal the JSON data
	jsonData, err := json.MarshalIndent(jsonOutput, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	// Write the JSON data to the file
	if err := os.WriteFile(outputPath, jsonData, 0600); err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	return nil
}

// versionPrinter prints the version information with more details than the default cli version printer.
// It uses consistent styling defined by the project's standards.
func versionPrinter(c *cli.Context) {
	summaryStyle := color.New(color.FgCyan, color.Bold)
	valueStyle := color.New(color.Bold)
	regularStyle := color.New(color.Reset)

	summaryStyle.Printf("🐾 FrameHound %s\n", Version)
	regularStyle.Printf("  🛠️ Build date: ")
	valueStyle.Printf("%s\n", BuildDate)
	regularStyle.Printf("  🔍 Commit: ")
	valueStyle.Printf("%s\n", Commit)
}

// Public functions (alphabetical)

// analyzeCommand implements the default command which analyzes a video file.
// It reads the video file, extracts container information, and outputs the results.
func analyzeCommand(c *cli.Context) error {
	valueStyle := color.New(color.Bold)
	regularStyle := color.New(color.Reset)
	successStyle := color.New(color.FgGreen)

	// Get the file path from the first argument
	if c.NArg() < 1 {
		return fmt.Errorf("missing required argument: VIDEO_FILE")
	}
	filePath := c.Args().Get(0)
	outputDir := c.String("dir")

	// Convert to absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("error resolving path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", absPath)
	}

	// Delete the output directory if it exists
	if _, err := os.Stat(outputDir); err == nil {
		if err := os.RemoveAll(outputDir); err != nil {
			return fmt.Errorf("error removing existing output directory: %w", err)
		}
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}

	// Find FFmpeg and check version
	ffmpegInfo, err := ffmpeg.FindFFmpeg()
	if err != nil {
		return fmt.Errorf("error finding FFmpeg: %w", err)
	}

	// Print FFmpeg information
	regularStyle.Printf("🔧 Using FFmpeg at ")
	valueStyle.Printf("%s\n", ffmpegInfo.Path)
	regularStyle.Printf("🔖 FFmpeg version: ")
	valueStyle.Printf("%s\n", ffmpegInfo.Version)

	// Create a prober for getting media information
	prober, err := ffmpeg.NewProber(ffmpegInfo)
	if err != nil {
		return fmt.Errorf("error creating prober: %w", err)
	}

	// Get detailed container information
	containerInfo, err := prober.GetExtendedContainerInfo(absPath)
	if err != nil {
		return fmt.Errorf("container not recognized: %w", err)
	}

	// Print container information to stdout
	printContainerInfo(containerInfo)

	// Save container information to a plain text file in the output directory
	if err := saveContainerInfo(containerInfo, outputDir); err != nil {
		return fmt.Errorf("error saving container info: %w", err)
	}

	successStyle.Printf("\n✅ Container information saved to %s\n", outputDir)
	return nil
}

// main is the entry point of the application.
// It parses command-line arguments, validates input, and starts the analysis.
func main() {
	// Override the default version printer
	cli.VersionPrinter = versionPrinter

	// Create a new CLI app
	app := &cli.App{
		Name:  "framehound",
		Usage: "A tool for analyzing video frame bitrates",
		Description: "FrameHound analyzes video files to extract frame-by-frame bitrate information " +
			"and provides detailed metadata about media containers.",
		Authors: []*cli.Author{
			{
				Name: "Gian Luca Dalla Torre",
			},
		},
		Version:   Version,
		Action:    analyzeCommand,
		ArgsUsage: "VIDEO_FILE",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "dir",
				Aliases: []string{"d"},
				Usage:   "Directory where to output the results of analysis",
				Value:   filepath.Join(".", "reports"),
			},
		},
	}

	// Run the application
	if err := app.Run(os.Args); err != nil {
		errorStyle := color.New(color.FgRed)
		errorStyle.Fprintf(os.Stderr, "⚠️ Error: %v\n", err)
		os.Exit(1)
	}
}
