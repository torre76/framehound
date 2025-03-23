// Package main provides the entry point for the framehound application.
// It analyzes video files to extract frame-by-frame bitrate information and
// provides comprehensive container information analysis.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/gertd/go-pluralize"
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
	// Convert the number to a string
	inStr := strconv.FormatInt(n, 10)

	// If the number is negative, handle the sign separately
	sign := ""
	if n < 0 {
		sign = "-"
		inStr = inStr[1:] // Remove the negative sign for processing
	}

	// Add thousand separators
	var result strings.Builder
	for i, c := range inStr {
		if i > 0 && (len(inStr)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(c)
	}

	// Add back the sign if needed
	return sign + result.String()
}

// printSimpleContainerSummary prints a simplified summary of the container information.
// It displays the file name and counts of video, audio, and subtitle streams
// with proper pluralization.
func printSimpleContainerSummary(info *ffmpeg.ContainerInfo, prober *ffmpeg.Prober) {
	// Initialize styles according to the go-stdout rules
	summaryStyle := color.New(color.FgCyan, color.Bold)
	valueStyle := color.New(color.Bold)
	regularStyle := color.New(color.Reset)

	// Initialize pluralize client
	pluralizeClient := pluralize.NewClient()

	// Get the filename from the Tags map
	fileName := ""
	if info.General.Tags != nil {
		if path, ok := info.General.Tags["file_path"]; ok {
			fileName = filepath.Base(path)
		}
	}

	// Get the container title using the Prober
	containerTitle := prober.GetContainerTitle(info)

	// Print the file name with proper styling
	summaryStyle.Println("📊 FILE ANALYSIS")
	regularStyle.Println("----------------")
	fmt.Println()
	regularStyle.Printf("🎬 Working on: ")
	valueStyle.Printf("%s [%s]\n", containerTitle, fileName)

	// Count the streams
	videoCount := len(info.VideoStreams)
	audioCount := len(info.AudioStreams)
	subtitleCount := len(info.SubtitleStreams)

	// Print the stream counts with proper pluralization
	summaryStyle.Println("\nℹ️ STREAM SUMMARY")
	regularStyle.Println("----------------")

	// Video streams
	regularStyle.Printf("🎞️ %d ", videoCount)
	valueStyle.Println(pluralizeClient.Pluralize("video stream", videoCount, false))

	// Audio streams
	regularStyle.Printf("🔊 %d ", audioCount)
	valueStyle.Println(pluralizeClient.Pluralize("audio stream", audioCount, false))

	// Subtitle streams
	regularStyle.Printf("💬 %d ", subtitleCount)
	valueStyle.Println(pluralizeClient.Pluralize("subtitle track", subtitleCount, false))
}

// parseBitRate parses the bitrate string and returns the value in bits per second.
func parseBitRate(bitRateStr string) int64 {
	bitRate := int64(0)
	if bitRateStr == "" {
		return bitRate
	}

	parts := strings.Fields(bitRateStr)
	if len(parts) < 1 {
		return bitRate
	}

	// Handle formats like "5 000" by removing spaces
	valueStr := strings.ReplaceAll(parts[0], " ", "")
	parsedBitRate, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return bitRate
	}

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

	return parsedBitRate
}

// getFrameRate extracts the frame rate from the first video stream if available.
func getFrameRate(info *ffmpeg.ContainerInfo) float64 {
	// Default frame rate if none found
	frameRate := 0.0

	// Get frame rate from video streams
	if len(info.VideoStreams) > 0 {
		// Try to find the first non-zero frame rate
		for _, stream := range info.VideoStreams {
			if stream.FrameRate > 0 {
				frameRate = stream.FrameRate
				break
			}
		}
	}

	return frameRate
}

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

// formatDuration formats seconds into a human-readable duration string
// such as "10.5 seconds" or "1 hour, 2 minutes and 13 seconds"
func formatDuration(seconds float64) string {
	// Return seconds with appropriate formatting if less than 60 seconds
	if seconds < 60 {
		// Check if it's a whole number
		if seconds == float64(int(seconds)) {
			return fmt.Sprintf("%d seconds", int(seconds))
		}
		return fmt.Sprintf("%.3f seconds", seconds)
	}

	// For longer durations, use humanize package to format it
	duration := time.Duration(seconds * float64(time.Second))
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	secs := int(duration.Seconds()) % 60

	var parts []string
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, "1 hour")
		} else {
			parts = append(parts, fmt.Sprintf("%d hours", hours))
		}
	}
	if minutes > 0 {
		if minutes == 1 {
			parts = append(parts, "1 minute")
		} else {
			parts = append(parts, fmt.Sprintf("%d minutes", minutes))
		}
	}
	if secs > 0 || (hours == 0 && minutes == 0) {
		if secs == 1 {
			parts = append(parts, "1 second")
		} else {
			parts = append(parts, fmt.Sprintf("%d seconds", secs))
		}
	}

	switch len(parts) {
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	case 3:
		return parts[0] + ", " + parts[1] + " and " + parts[2]
	default:
		return fmt.Sprintf("%.3f seconds", seconds)
	}
}

// writeMediaInfoHeader writes the header section of the media info file
func writeMediaInfoHeader(w *tabwriter.Writer, containerTitle, fileName string, videoCount, audioCount, subtitleCount int) {
	pluralizeClient := pluralize.NewClient()

	// Write summary header
	fmt.Fprintln(w, "===========================================")
	fmt.Fprintln(w, "MEDIA INFORMATION SUMMARY")
	fmt.Fprintln(w, "===========================================")
	fmt.Fprintln(w)

	// Title and filename
	fmt.Fprintf(w, "Title:\t%s\n", containerTitle)
	fmt.Fprintf(w, "Filename:\t%s\n", fileName)
	fmt.Fprintln(w)

	// Stream counts
	fmt.Fprintf(w, "Streams:\t%d %s, %d %s, %d %s\n",
		videoCount, pluralizeClient.Pluralize("video stream", videoCount, false),
		audioCount, pluralizeClient.Pluralize("audio stream", audioCount, false),
		subtitleCount, pluralizeClient.Pluralize("subtitle track", subtitleCount, false))
	fmt.Fprintln(w)
}

// writeMediaInfoBasicData writes the basic container data section
func writeMediaInfoBasicData(w *tabwriter.Writer, info *ffmpeg.ContainerInfo) {
	// Calculate bitrate if not available
	bitRate := parseBitRate(info.General.BitRate)
	if bitRate == 0 && info.General.DurationF > 0 {
		sizeBytes, err := strconv.ParseInt(strings.Fields(info.General.Size)[0], 10, 64)
		if err == nil && sizeBytes > 0 {
			bitRate = int64(float64(sizeBytes*8) / info.General.DurationF)
		}
	}

	// Format size with human-readable form
	sizeInBytes := int64(0)
	if sizeFields := strings.Fields(info.General.Size); len(sizeFields) > 0 {
		if parsedSize, err := strconv.ParseInt(sizeFields[0], 10, 64); err == nil {
			sizeInBytes = parsedSize
		}
	}
	humanSize := formatHumanReadableSize(int(sizeInBytes))

	// Write bitrate and size
	fmt.Fprintf(w, "Bitrate:\t%.2f Kbps\n", float64(bitRate)/1000)
	fmt.Fprintf(w, "Size:\t%s\n", humanSize)
	fmt.Fprintln(w)
}

// writeMediaInfoContainerSection writes the container information section
func writeMediaInfoContainerSection(w *tabwriter.Writer, info *ffmpeg.ContainerInfo) {
	fmt.Fprintln(w, "===========================================")
	fmt.Fprintln(w, "CONTAINER INFORMATION")
	fmt.Fprintln(w, "===========================================")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "Format:\t%s\n", info.General.Format)

	// Format the duration both as seconds and human-readable form
	humanDuration := formatDuration(info.General.DurationF)

	// Format seconds as integer if it's a whole number
	var durationStr string
	if info.General.DurationF == float64(int(info.General.DurationF)) {
		durationStr = fmt.Sprintf("%d seconds", int(info.General.DurationF))
	} else {
		durationStr = fmt.Sprintf("%.3f seconds", info.General.DurationF)
	}

	fmt.Fprintf(w, "Duration:\t%s (%s)\n", durationStr, humanDuration)

	// Write tags if available
	if len(info.General.Tags) > 0 {
		fmt.Fprintln(w, "\nTags:")
		for key, value := range info.General.Tags {
			if key != "file_path" { // Skip file_path as we already displayed it
				fmt.Fprintf(w, "  %s:\t%s\n", key, value)
			}
		}
	}
	fmt.Fprintln(w)
}

// writeMediaInfoVideoStreams writes video stream information
func writeMediaInfoVideoStreams(w *tabwriter.Writer, streams []ffmpeg.VideoStream, info *ffmpeg.ContainerInfo) {
	if len(streams) == 0 {
		return
	}

	fmt.Fprintln(w, "===========================================")
	fmt.Fprintln(w, "VIDEO STREAMS")
	fmt.Fprintln(w, "===========================================")

	// Calculate total audio bitrate
	totalAudioBitrate := int64(0)
	for _, audio := range info.AudioStreams {
		totalAudioBitrate += audio.BitRate
	}

	// Get container bitrate
	containerBitrate := parseBitRate(info.General.BitRate)

	for i, stream := range streams {
		fmt.Fprintf(w, "\nStream #%d:\n", i)
		fmt.Fprintf(w, "  Codec:\t%s\n", stream.Format)

		if stream.FormatProfile != "" {
			fmt.Fprintf(w, "  Codec Profile:\t%s\n", stream.FormatProfile)
		}

		if stream.Title != "" {
			fmt.Fprintf(w, "  Title:\t%s\n", stream.Title)
		}

		fmt.Fprintf(w, "  Resolution:\t%dx%d pixels\n", stream.Width, stream.Height)
		if stream.DisplayAspectRatio > 0 {
			fmt.Fprintf(w, "  Aspect Ratio:\t%.3f\n", stream.DisplayAspectRatio)
		}
		fmt.Fprintf(w, "  Frame Rate:\t%.3f fps\n", stream.FrameRate)

		// Handle bitrate display with calculation if missing
		videoBitrate := stream.BitRate
		if videoBitrate <= 0 && len(streams) == 1 && containerBitrate > 0 {
			// For a single video stream, estimate bitrate by subtracting audio from container bitrate
			estimatedBitrate := containerBitrate - totalAudioBitrate
			if estimatedBitrate > 0 {
				fmt.Fprintf(w, "  Bit Rate:\t%.2f Kbps (estimated)\n", float64(estimatedBitrate)/1000)
			}
			// Don't display anything if bitrate can't be calculated
		} else if videoBitrate > 0 {
			fmt.Fprintf(w, "  Bit Rate:\t%.2f Kbps\n", float64(videoBitrate)/1000)
		}
		// Don't display bitrate at all if it's not available or can't be calculated

		fmt.Fprintf(w, "  Bit Depth:\t%d bits\n", stream.BitDepth)

		if stream.ColorSpace != "" {
			fmt.Fprintf(w, "  Color Space:\t%s\n", stream.ColorSpace)
		}

		if stream.ScanType != "" {
			fmt.Fprintf(w, "  Scan Type:\t%s\n", stream.ScanType)
		}

		if stream.Language != "" {
			fmt.Fprintf(w, "  Language:\t%s\n", stream.Language)
		}
	}
	fmt.Fprintln(w)
}

// writeMediaInfoAudioStreams writes audio stream information
func writeMediaInfoAudioStreams(w *tabwriter.Writer, streams []ffmpeg.AudioStream) {
	if len(streams) == 0 {
		return
	}

	fmt.Fprintln(w, "===========================================")
	fmt.Fprintln(w, "AUDIO STREAMS")
	fmt.Fprintln(w, "===========================================")

	for i, stream := range streams {
		fmt.Fprintf(w, "\nStream #%d:\n", i)
		fmt.Fprintf(w, "  Codec:\t%s\n", stream.Format)

		if stream.Title != "" {
			fmt.Fprintf(w, "  Title:\t%s\n", stream.Title)
		}

		fmt.Fprintf(w, "  Channels:\t%d", stream.Channels)
		if stream.ChannelLayout != "" {
			fmt.Fprintf(w, " (%s)", stream.ChannelLayout)
		}
		fmt.Fprintln(w)

		fmt.Fprintf(w, "  Sampling Rate:\t%d Hz\n", stream.SamplingRate)

		if stream.BitRate > 0 {
			fmt.Fprintf(w, "  Bit Rate:\t%.2f Kbps\n", float64(stream.BitRate)/1000)
		}

		if stream.Language != "" {
			fmt.Fprintf(w, "  Language:\t%s\n", stream.Language)
		}
	}
	fmt.Fprintln(w)
}

// writeMediaInfoSubtitleStreams writes subtitle stream information
func writeMediaInfoSubtitleStreams(w *tabwriter.Writer, streams []ffmpeg.SubtitleStream) {
	if len(streams) == 0 {
		return
	}

	fmt.Fprintln(w, "===========================================")
	fmt.Fprintln(w, "SUBTITLE STREAMS")
	fmt.Fprintln(w, "===========================================")

	for i, stream := range streams {
		fmt.Fprintf(w, "\nStream #%d:\n", i)
		fmt.Fprintf(w, "  Format:\t%s\n", stream.Format)

		if stream.Title != "" {
			fmt.Fprintf(w, "  Title:\t%s\n", stream.Title)
		}

		if stream.Language != "" {
			fmt.Fprintf(w, "  Language:\t%s\n", stream.Language)
		}
	}
	fmt.Fprintln(w)
}

// writeMediaInfoChapters writes chapter information
func writeMediaInfoChapters(w *tabwriter.Writer, chapters []ffmpeg.ChapterStream) {
	if len(chapters) == 0 {
		return
	}

	fmt.Fprintln(w, "===========================================")
	fmt.Fprintln(w, "CHAPTERS")
	fmt.Fprintln(w, "===========================================")

	for _, chapter := range chapters {
		fmt.Fprintf(w, "\nChapter #%d:\n", chapter.ID)
		if chapter.Title != "" {
			fmt.Fprintf(w, "  Title:\t%s\n", chapter.Title)
		}

		// Format start and end times
		var startTimeStr, endTimeStr string
		if chapter.StartTime == float64(int(chapter.StartTime)) {
			startTimeStr = fmt.Sprintf("%d seconds", int(chapter.StartTime))
		} else {
			startTimeStr = fmt.Sprintf("%.3f seconds", chapter.StartTime)
		}

		if chapter.EndTime == float64(int(chapter.EndTime)) {
			endTimeStr = fmt.Sprintf("%d seconds", int(chapter.EndTime))
		} else {
			endTimeStr = fmt.Sprintf("%.3f seconds", chapter.EndTime)
		}

		fmt.Fprintf(w, "  Start Time:\t%s\n", startTimeStr)
		fmt.Fprintf(w, "  End Time:\t%s\n", endTimeStr)

		// Format chapter duration in human-readable form
		chapterDuration := chapter.EndTime - chapter.StartTime
		humanDuration := formatDuration(chapterDuration)

		// Format duration seconds as integer if it's a whole number
		var durationStr string
		if chapterDuration == float64(int(chapterDuration)) {
			durationStr = fmt.Sprintf("%d seconds", int(chapterDuration))
		} else {
			durationStr = fmt.Sprintf("%.3f seconds", chapterDuration)
		}

		fmt.Fprintf(w, "  Duration:\t%s (%s)\n", durationStr, humanDuration)
	}
	fmt.Fprintln(w)
}

// writeMediaInfoAttachments writes attachment information
func writeMediaInfoAttachments(w *tabwriter.Writer, attachments []ffmpeg.AttachmentStream) {
	if len(attachments) == 0 {
		return
	}

	fmt.Fprintln(w, "===========================================")
	fmt.Fprintln(w, "ATTACHMENTS")
	fmt.Fprintln(w, "===========================================")

	for i, attachment := range attachments {
		fmt.Fprintf(w, "\nAttachment #%d:\n", i+1)
		if attachment.FileName != "" {
			fmt.Fprintf(w, "  Filename:\t%s\n", attachment.FileName)
		}
		if attachment.MimeType != "" {
			fmt.Fprintf(w, "  MIME Type:\t%s\n", attachment.MimeType)
		}
	}
	fmt.Fprintln(w)
}

// writeMediaInfoFooter writes the footer with metadata about when the report was generated
func writeMediaInfoFooter(w *tabwriter.Writer) {
	fmt.Fprintln(w, "===========================================")
	fmt.Fprintf(w, "Analysis Generated: %s\n", time.Now().Format(time.RFC1123))
	fmt.Fprintf(w, "FrameHound Version: %s\n", Version)
	fmt.Fprintln(w, "===========================================")
}

// saveMediaInfoText saves detailed container information to a text file in the specified directory.
// It includes comprehensive information about the container and all streams.
func saveMediaInfoText(info *ffmpeg.ContainerInfo, outputDir string, prober *ffmpeg.Prober) error {
	// Create the output directory if it doesn't exist
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		return fmt.Errorf("error creating output directory: %w", err)
	}

	// Define output file path
	outputPath := filepath.Join(outputDir, "mediainfo.txt")

	// Create the file
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("error creating mediainfo file: %w", err)
	}
	defer file.Close()

	// Initialize tabwriter for better formatting
	w := tabwriter.NewWriter(file, 0, 0, 2, ' ', tabwriter.StripEscape)

	// Get the container title
	containerTitle := prober.GetContainerTitle(info)

	// Get the filename from the Tags map
	fileName := ""
	if info.General.Tags != nil {
		if path, ok := info.General.Tags["file_path"]; ok {
			fileName = filepath.Base(path)
		}
	}

	// Count streams
	videoCount := len(info.VideoStreams)
	audioCount := len(info.AudioStreams)
	subtitleCount := len(info.SubtitleStreams)

	// Write the different sections of the report
	writeMediaInfoHeader(w, containerTitle, fileName, videoCount, audioCount, subtitleCount)
	writeMediaInfoBasicData(w, info)
	writeMediaInfoContainerSection(w, info)
	writeMediaInfoVideoStreams(w, info.VideoStreams, info)
	writeMediaInfoAudioStreams(w, info.AudioStreams)
	writeMediaInfoSubtitleStreams(w, info.SubtitleStreams)
	writeMediaInfoChapters(w, info.ChapterStreams)
	writeMediaInfoAttachments(w, info.AttachmentStreams)
	writeMediaInfoFooter(w)

	// Flush buffered data to ensure it's written to the file
	if err := w.Flush(); err != nil {
		return fmt.Errorf("error flushing output: %w", err)
	}

	return nil
}

// formatHumanReadableSize formats a size in bytes to a human-readable format
func formatHumanReadableSize(bytes int) string {
	const (
		_          = iota
		KB float64 = 1 << (10 * iota)
		MB
		GB
		TB
	)

	if bytes < 1000 {
		return fmt.Sprintf("%d bytes", bytes)
	} else if bytes < 1000*int(KB) {
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	} else if bytes < 1000*int(MB) {
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	} else if bytes < 1000*int(GB) {
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	}
	return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
}

// Public functions (alphabetical)

// analyzeCommand implements the default command which analyzes a video file.
// It reads the video file, extracts container information, and outputs the results.
func analyzeCommand(c *cli.Context) error {
	valueStyle := color.New(color.Bold)
	regularStyle := color.New(color.Reset)
	successStyle := color.New(color.FgGreen)
	errorStyle := color.New(color.FgRed)

	// Get the file path from the first argument
	if c.NArg() < 1 {
		// Display a more user-friendly message and usage information
		errorStyle.Printf("❌ Error: missing required argument: VIDEO_FILE\n\n")
		regularStyle.Printf("Usage: %s [options] VIDEO_FILE\n", c.App.Name)
		regularStyle.Printf("Run '%s --help' for more information.\n", c.App.Name)
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
	valueStyle.Printf("%s\n\n", ffmpegInfo.Version)

	// Create a prober for getting media information
	prober, err := ffmpeg.NewProber(ffmpegInfo)
	if err != nil {
		return fmt.Errorf("error creating prober: %w", err)
	}

	// Get detailed container information
	containerInfo, err := prober.GetExtendedContainerInfo(absPath)
	if err != nil {
		errorStyle.Printf("❌ Container not recognized: %v\n", err)
		return fmt.Errorf("container not recognized: %w", err)
	}

	// Print simplified container summary with container title
	printSimpleContainerSummary(containerInfo, prober)

	// Save detailed media information to a text file in the output directory
	if err := saveMediaInfoText(containerInfo, outputDir, prober); err != nil {
		return fmt.Errorf("error saving media info text: %w", err)
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
