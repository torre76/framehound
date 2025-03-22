// Package ffmpeg provides functionality for interacting with FFmpeg
// and extracting information from media files.
package ffmpeg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetContainerTitle tests the GetContainerTitle method.
func TestGetContainerTitle(t *testing.T) {
	// Create a prober for testing
	prober := &Prober{
		FFmpegInfo: &FFmpegInfo{
			Installed: true,
			Path:      "/usr/bin/ffmpeg",
			Version:   "4.2.2",
		},
	}

	tests := []struct {
		name     string
		info     *ContainerInfo
		expected string
	}{
		{
			name: "Title from metadata",
			info: &ContainerInfo{
				General: GeneralInfo{
					Format: "MPEG-4",
					Tags: map[string]string{
						"title": "Sample Video Title",
					},
				},
			},
			expected: "Sample Video Title",
		},
		{
			name: "Title from TITLE tag (uppercase)",
			info: &ContainerInfo{
				General: GeneralInfo{
					Format: "MPEG-4",
					Tags: map[string]string{
						"TITLE": "Sample Video Title Uppercase",
					},
				},
			},
			expected: "Untitled Media",
		},
		{
			name: "Title from filename",
			info: &ContainerInfo{
				General: GeneralInfo{
					Format: "MPEG-4",
					Tags: map[string]string{
						"file_path": "/path/to/The.Matrix.1999.1080p.BluRay.x264.mkv",
					},
				},
			},
			expected: "The Matrix 1999 1080p Bluray",
		},
		{
			name: "No title",
			info: &ContainerInfo{
				General: GeneralInfo{
					Format: "MPEG-4",
					Tags:   map[string]string{},
				},
			},
			expected: "Untitled Media",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prober.GetContainerTitle(tt.info)
			assert.Equal(t, tt.expected, got)
		})
	}
}
