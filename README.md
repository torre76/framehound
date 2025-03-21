# FrameHound

<p align="center">
  <img src="resources/images/framehound.jpg" width="50%" alt="FrameHound">
</p>

<p align="center">
  <a href="https://github.com/torre76/framehound/actions/workflows/ci.yml"><img src="https://github.com/torre76/framehound/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://codecov.io/gh/torre76/framehound"><img src="https://codecov.io/gh/torre76/framehound/branch/main/graph/badge.svg" alt="codecov"></a>
  <a href="https://goreportcard.com/report/github.com/torre76/framehound"><img src="https://goreportcard.com/badge/github.com/torre76/framehound" alt="Go Report Card"></a>
</p>

FrameHound is a tool for analyzing video files, with a focus on frame-level metrics and quality assessment.

## Features

- Automatic detection of FFmpeg installation
- Support for QP (Quantization Parameter) analysis of video files
- Support for CU (Coding Unit) analysis for advanced codec debugging
- Real-time processing of video frames
- Detailed frame rate analysis with improved precision (to 3 decimal places)
- Codec support for QP analysis: XviD, DivX, AVI, and H.264 (not AVC)

## Requirements

- FFmpeg with debug support (compiled with `--enable-debug` or with a debug build available)
- Go 1.16 or higher

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

### Basic Usage

```bash
framehound VIDEO_FILE
```

This will analyze the video file and generate a report in the `./reports` directory.

### Custom Output Directory

```bash
framehound --dir=custom_reports VIDEO_FILE
```

or use the short flag:

```bash
framehound -d custom_reports VIDEO_FILE
```

### Version Information

To view version information:

```bash
framehound --version
```

## Output

FrameHound provides detailed information about video files, including:

### Container Information

- Format and format version
- File size
- Duration
- Overall bitrate
- Frame rate

### Video Streams

- Codec and profile
- Resolution
- Aspect ratio
- Bit depth
- Bitrate
- Frame rate
- Scan type
- Color space

### Audio Streams

- Codec
- Channels and layout
- Sample rate
- Bitrate
- Language

### Subtitle Streams

- Codec
- Language
- Title

## Development

### Project Structure

The project is organized into the following main packages:

- **ffmpeg**: Contains all functionality related to ffmpeg operations
- **main**: Entry point for the command-line application

### Continuous Integration

This project uses GitHub Actions for continuous integration with the following checks:

- **Testing**: Runs the unit test suite with race detection
- **Code Coverage**: Reports test coverage to Codecov
- **Static Analysis**: Uses golangci-lint to check for common issues
- **Dependency Scanning**: Checks for vulnerable dependencies using govulncheck and deps.dev

To run the same checks locally:

```bash
# Run tests with coverage
go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

# Install and run golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
golangci-lint run

# Check for vulnerabilities
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

### Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Commit your changes: `git commit -am 'Add some feature'`
4. Push to the branch: `git push origin feature/my-feature`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgements

- FFmpeg team for their incredible video processing tools
- The Go community for the excellent standard library

## Contact

For questions, issues, or feature requests, please open an issue on GitHub.
