# FrameHound

<p align="center">
  <img src="resources/images/framehound.jpg" width="50%" alt="FrameHound">
</p>

<p align="center">
  <a href="https://github.com/torre76/framehound/actions/workflows/ci.yml"><img src="https://github.com/torre76/framehound/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/torre76/framehound"><img src="https://codecov.io/gh/torre76/framehound/branch/main/graph/badge.svg" alt="codecov"></a>
  <a href="https://goreportcard.com/report/github.com/torre76/framehound"><img src="https://goreportcard.com/badge/github.com/torre76/framehound" alt="Go Report Card"></a>
</p>

A tool for analyzing video frame bitrates and extracting detailed media container information.

## Features

- **Media Container Analysis**: Extract comprehensive metadata about media containers
- **Bitrate Analysis**: Frame-by-frame bitrate information with CSV output
- **Detailed Reports**: Generate detailed text reports with file and stream information
- **Progress Tracking**: Visual progress bars with ETA during analysis
- **User-Friendly Output**: Clean, color-coded terminal output

## Requirements

- FFmpeg installed on your system (version 4.0 or newer recommended)
- Go 1.20+ for building from source

## Installation

### Using Go Install

```bash
go install github.com/torre76/framehound@latest
```

### Building from Source

The project includes a Makefile to simplify building:

```bash
# Clone the repository
git clone https://github.com/torre76/framehound.git
cd framehound

# Show available make targets
make help

# Download dependencies
make deps

# Build with version from git tags
# The executable will be placed in the ./dist directory
make build

# Clean and rebuild
make clean build

# Install to your GOPATH/bin
make install
```

## Usage

### Command Line Interface

FrameHound is a command-line tool that analyzes video files and generates reports:

```bash
# Basic usage
framehound VIDEO_FILE

# Specify custom output directory
framehound --dir=my-reports VIDEO_FILE
framehound -d my-reports VIDEO_FILE

# Show detailed frame count information (debugging)
framehound --show-frames VIDEO_FILE

# Show version information
framehound --version
framehound -v
```

### Output

FrameHound generates the following outputs in the reports directory:

1. `mediainfo.txt`: Detailed text report with comprehensive media information
2. `bitrate.csv`: CSV file with frame-by-frame bitrate information

Example output:

```
🔧 Using FFmpeg at /usr/bin/ffmpeg
🔖 FFmpeg version: 5.1.2

📊 FILE ANALYSIS
----------------

🎬 Working on: Sample Video [sample.mkv]

ℹ️ STREAM SUMMARY
----------------
🎞️ 1 video stream
🔊 2 audio streams
💬 0 subtitle tracks
✅ Media information saved to reports/mediainfo.txt

🔍 BITRATE ANALYSIS
----------------

📈 Generating bitrate report - Completed!
✅ Bitrate report saved to reports/bitrate.csv

✅ Analysis complete! All reports saved to reports
```

### Programmatic Usage

FrameHound can also be used as a library in your Go projects:

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/torre76/framehound/ffmpeg"
)

func main() {
    // Detect FFmpeg
    ffmpegInfo, err := ffmpeg.DetectFFmpeg()
    if err != nil {
        log.Fatalf("Failed to detect FFmpeg: %v", err)
    }
    
    fmt.Printf("FFmpeg found at: %s\n", ffmpegInfo.Path)
    fmt.Printf("FFmpeg version: %s\n", ffmpegInfo.Version)
    
    // Create a prober for container analysis
    prober, err := ffmpeg.NewProber(ffmpegInfo)
    if err != nil {
        log.Fatalf("Failed to create prober: %v", err)
    }
    
    // Get container information
    videoFile := "path/to/your/video.mp4"
    containerInfo, err := prober.GetExtendedContainerInfo(videoFile)
    if err != nil {
        log.Fatalf("Failed to analyze file: %v", err)
    }
    
    // Print basic information
    fmt.Printf("Format: %s\n", containerInfo.General.Format)
    fmt.Printf("Duration: %.2f seconds\n", containerInfo.General.DurationF)
    fmt.Printf("Video streams: %d\n", len(containerInfo.VideoStreams))
    fmt.Printf("Audio streams: %d\n", len(containerInfo.AudioStreams))
    
    // Create a bitrate analyzer
    bitrateAnalyzer, err := ffmpeg.NewBitrateAnalyzer(ffmpegInfo)
    if err != nil {
        log.Fatalf("Failed to create bitrate analyzer: %v", err)
    }
    
    // Analyze bitrate information
    // Implementation details depend on specific needs
}
```

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgements

- FFmpeg team for their incredible video processing tools
- The Go community for the excellent standard library

## Contact

For questions, issues, or feature requests, please open an issue on GitHub.
