# FrameHound

<p align="center">
  <img src="resources/images/framehound.jpg" width="50%" alt="FrameHound">
</p>

<p align="center">
  <a href="https://github.com/torre76/framehound/actions/workflows/ci.yml"><img src="https://github.com/torre76/framehound/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/torre76/framehound"><img src="https://codecov.io/gh/torre76/framehound/branch/main/graph/badge.svg" alt="codecov"></a>
  <a href="https://goreportcard.com/report/github.com/torre76/framehound"><img src="https://goreportcard.com/badge/github.com/torre76/framehound" alt="Go Report Card"></a>
</p>

Video analysis toolkit for extracting quality metrics and frame information from video files.

## Features

- **QP Analyzer**: Extract Quantization Parameter (QP) values from H.264, HEVC, and other compatible video files
- **Bitrate Analyzer**: Analyze video bitrate statistics
- **Quality Analyzer**: Comprehensive video quality analysis including:
  - Basic video properties (codec, resolution, duration)
  - Quality metrics (PSNR, SSIM, VMAF)
  - QP analysis for supported codecs
  - Frame-level information

## Requirements

- FFmpeg and FFprobe installed on your system
- Go 1.16+

## Installation

### Using Go Install

```bash
go get github.com/torre76/framehound
```

### Building from Source

#### Standard Build

To build FrameHound from source:

```bash
git clone https://github.com/torre76/framehound.git
cd framehound
go build
```

#### Building with Version Information

FrameHound uses build-time variables to embed version information. The Makefile provides targets to simplify this process:

```bash
# Download dependencies only
make deps

# Build with version from git tags (automatically downloads dependencies)
# The executable will be placed in the ./dist directory
make build

# Clean and rebuild
make clean build
```

## Usage

### Quality Analysis

Analyze video quality metrics of a file:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "github.com/youruser/framehound/ffmpeg"
)

func main() {
    // Create a context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    
    // Get FFmpeg info
    ffmpegInfo, err := ffmpeg.NewFFmpegInfo()
    if err != nil {
        log.Fatalf("Failed to get FFmpeg info: %v", err)
    }
    
    // Create a quality analyzer
    analyzer := ffmpeg.NewQualityAnalyzer(ffmpegInfo.FFmpegPath)
    
    // Path to video file
    videoFile := "path/to/your/video.mp4"
    
    // Generate quality report
    report, err := analyzer.GenerateQualityReport(ctx, videoFile)
    if err != nil {
        log.Fatalf("Failed to generate quality report: %v", err)
    }
    
    // Print report
    fmt.Printf("Video Quality Report:\n")
    fmt.Printf("Format: %s\n", report.Format)
    fmt.Printf("Codec: %s\n", report.Codec)
    fmt.Printf("Resolution: %dx%d\n", report.Width, report.Height)
    fmt.Printf("Duration: %.2f seconds\n", report.Duration)
    fmt.Printf("Bitrate: %d kbps\n", report.Bitrate)
    
    if report.AverageQP > 0 {
        fmt.Printf("Average QP: %.2f\n", report.AverageQP)
    }
    
    // Calculate PSNR, SSIM, and VMAF against a reference file
    referenceFile := "path/to/reference.mp4"
    
    psnr, err := analyzer.CalculatePSNR(ctx, videoFile, referenceFile)
    if err == nil {
        fmt.Printf("PSNR: %.2f dB\n", psnr)
    }
    
    ssim, err := analyzer.CalculateSSIM(ctx, videoFile, referenceFile)
    if err == nil {
        fmt.Printf("SSIM: %.4f\n", ssim)
    }
    
    vmaf, err := analyzer.CalculateVMAF(ctx, videoFile, referenceFile)
    if err == nil {
        fmt.Printf("VMAF: %.2f\n", vmaf)
    }
}
```

### Running Quality Tests

The project includes a script to run quality tests:

```bash
# Run all quality tests
./scripts/run_quality_tests.sh

# Run tests for a specific sample
./scripts/run_quality_tests.sh 1  # For sample.mkv
./scripts/run_quality_tests.sh 2  # For sample2.mkv
./scripts/run_quality_tests.sh 3  # For sample3.avi
./scripts/run_quality_tests.sh 4  # For sample4.avi
```

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Acknowledgements

- FFmpeg team for their incredible video processing tools
- The Go community for the excellent standard library

## Contact

For questions, issues, or feature requests, please open an issue on GitHub.
