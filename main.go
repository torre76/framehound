// Package main provides the entry point for the framehound application.
// It analyzes video files to extract frame-by-frame bitrate information.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/torre76/framehound/ffmpeg"
	"github.com/urfave/cli/v2"
)

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

// printContainerInfo prints detailed information about the media container.
// It displays general information, video streams, audio streams, and subtitle streams.
func printContainerInfo(info *ffmpeg.ContainerInfo) {
	// Style definitions
	summaryStyle := color.New(color.FgCyan, color.Bold)
	valueStyle := color.New(color.Bold)
	regularStyle := color.New(color.Reset)

	summaryStyle.Println("\n📊 Container Information:")
	regularStyle.Println("---------------------")
	regularStyle.Printf("📄 File: ")
	valueStyle.Printf("%s\n", info.General.CompleteName)
	regularStyle.Printf("📦 Format: ")
	valueStyle.Printf("%s %s\n", info.General.Format, info.General.FormatVersion)
	regularStyle.Printf("💾 Size: ")
	valueStyle.Printf("%s bytes\n", formatWithThousandSeparators(info.General.FileSize))
	regularStyle.Printf("⏱️ Duration: ")
	valueStyle.Printf("%.3f seconds\n", info.General.Duration)
	regularStyle.Printf("⚡ Overall bitrate: ")
	valueStyle.Printf("%.2f Kbps\n", float64(info.General.OverallBitRate)/1000)
	regularStyle.Printf("🖼️ Frame rate: ")
	valueStyle.Printf("%.3f fps\n", info.General.FrameRate)

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
			valueStyle.Printf("%.0f Hz\n", stream.SamplingRate)
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

// formatWithThousandSeparators formats an integer with thousand separators.
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

// versionPrinter prints the version information with more details than the default cli version printer.
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

// analyzeCommand implements the default command which analyzes a video file
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
		return fmt.Errorf("error resolving path: %v", err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", absPath)
	}

	// Delete the output directory if it exists
	if _, err := os.Stat(outputDir); err == nil {
		if err := os.RemoveAll(outputDir); err != nil {
			return fmt.Errorf("error removing existing output directory: %v", err)
		}
	}

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("error creating output directory: %v", err)
	}

	// Find FFmpeg and check version
	ffmpegInfo, err := ffmpeg.FindFFmpeg()
	if err != nil {
		return fmt.Errorf("error finding FFmpeg: %v", err)
	}

	// Print FFmpeg information
	regularStyle.Printf("🔧 Using FFmpeg at ")
	valueStyle.Printf("%s\n", ffmpegInfo.Path)
	regularStyle.Printf("🔖 FFmpeg version: ")
	valueStyle.Printf("%s\n", ffmpegInfo.Version)

	// Create a prober for getting media information
	prober, err := ffmpeg.NewProber(ffmpegInfo)
	if err != nil {
		return fmt.Errorf("error creating prober: %v", err)
	}

	// Get detailed container information
	containerInfo, err := prober.GetExtendedContainerInfo(absPath)
	if err != nil {
		return fmt.Errorf("container not recognized: %v", err)
	}

	// Print container information to stdout
	printContainerInfo(containerInfo)

	// Save container information to a plain text file in the output directory
	if err := saveContainerInfo(containerInfo, outputDir); err != nil {
		return fmt.Errorf("error saving container info: %v", err)
	}

	successStyle.Printf("\n✅ Container information saved to %s\n", outputDir)
	return nil
}

// saveContainerInfo saves the container information to a plain text file in the specified directory
func saveContainerInfo(info *ffmpeg.ContainerInfo, outputDir string) error {
	// Create the output file path
	outputFile := filepath.Join(outputDir, "info.txt")

	// Create a string buffer to hold the output
	var output string

	// Format the same information as printContainerInfo but to a string
	output += "\n📊 Container Information:\n"
	output += "---------------------\n"
	output += fmt.Sprintf("📄 File: %s\n", info.General.CompleteName)
	output += fmt.Sprintf("📦 Format: %s %s\n", info.General.Format, info.General.FormatVersion)
	output += fmt.Sprintf("💾 Size: %s bytes\n", formatWithThousandSeparators(info.General.FileSize))
	output += fmt.Sprintf("⏱️ Duration: %.3f seconds\n", info.General.Duration)
	output += fmt.Sprintf("⚡ Overall bitrate: %.2f Kbps\n", float64(info.General.OverallBitRate)/1000)
	output += fmt.Sprintf("🖼️ Frame rate: %.3f fps\n", info.General.FrameRate)

	if len(info.VideoStreams) > 0 {
		output += "\n🎬 Video Streams:\n"
		output += "-------------\n"
		for i, stream := range info.VideoStreams {
			output += fmt.Sprintf("Stream #%d:\n", i)
			output += fmt.Sprintf("  🎞️ Codec: %s (%s)\n", stream.Format, stream.FormatProfile)
			output += fmt.Sprintf("  📐 Resolution: %dx%d pixels\n", stream.Width, stream.Height)
			output += fmt.Sprintf("  📺 Display Aspect Ratio: %.3f\n", stream.DisplayAspectRatio)
			output += fmt.Sprintf("  🔍 Bit depth: %d bits\n", stream.BitDepth)
			output += fmt.Sprintf("  ⚡ Bit rate: %.2f Kbps\n", float64(stream.BitRate)/1000)
			output += fmt.Sprintf("  🖼️ Frame rate: %.3f fps\n", stream.FrameRate)
			output += fmt.Sprintf("  📲 Scan type: %s\n", stream.ScanType)
			output += fmt.Sprintf("  🎨 Color space: %s\n", stream.ColorSpace)
		}
	}

	if len(info.AudioStreams) > 0 {
		output += "\n🔊 Audio Streams:\n"
		output += "-------------\n"
		for i, stream := range info.AudioStreams {
			output += fmt.Sprintf("Stream #%d:\n", i)
			output += fmt.Sprintf("  🎚️ Codec: %s\n", stream.Format)
			output += fmt.Sprintf("  🔈 Channels: %d (%s)\n", stream.Channels, stream.ChannelLayout)
			output += fmt.Sprintf("  📊 Sample rate: %.0f Hz\n", stream.SamplingRate)
			output += fmt.Sprintf("  ⚡ Bit rate: %.2f Kbps\n", float64(stream.BitRate)/1000)
			output += fmt.Sprintf("  🌐 Language: %s\n", stream.Language)
		}
	}

	if len(info.SubtitleStreams) > 0 {
		output += "\n💬 Subtitle Streams:\n"
		output += "----------------\n"
		for i, stream := range info.SubtitleStreams {
			output += fmt.Sprintf("Stream #%d:\n", i)
			output += fmt.Sprintf("  📝 Codec: %s\n", stream.Format)
			output += fmt.Sprintf("  🌐 Language: %s\n", stream.Language)
			output += fmt.Sprintf("  📌 Title: %s\n", stream.Title)
		}
	}

	// Add metadata about the analysis
	output += "\n🔎 Analysis Information:\n"
	output += "--------------------\n"
	output += fmt.Sprintf("🕒 Timestamp: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	output += fmt.Sprintf("🔖 FrameHound Version: %s\n", Version)
	output += fmt.Sprintf("🛠️ Build Date: %s\n", BuildDate)

	// Write the output to the file
	if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}

	return nil
}
