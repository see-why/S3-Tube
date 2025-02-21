package videoUtils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

type Stream struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type FfprobeOutput struct {
	Streams []Stream `json:"streams"`
}

// GetAspectRatio retrieves the aspect ratio of a video file
func GetAspectRatio(filePath string) (string, error) {
	// Prepare the ffprobe command
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run ffprobe: %w", err)
	}

	var ffprobeOutput FfprobeOutput
	if err := json.Unmarshal(out.Bytes(), &ffprobeOutput); err != nil {
		return "", fmt.Errorf("failed to unmarshal ffprobe output: %w", err)
	}

	if len(ffprobeOutput.Streams) == 0 {
		return "", fmt.Errorf("no streams found in video file")
	}

	width := ffprobeOutput.Streams[0].Width
	height := ffprobeOutput.Streams[0].Height

	if width == height {
		return "1:1", nil
	}

	if width/height == 16/9 {
		return "landscape", nil
	}

	if width/height == 0 {
		return "portrait", nil
	}

	return "other", nil
}

// ProcessForFastStart optimizes video for web playback
func ProcessForFastStart(filePath string) (string, error) {
	outputPath := fmt.Sprintf("%s.processing.mp4", filePath)
	err := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputPath).Run()

	if err != nil {
		return "", fmt.Errorf("failed to process video: %w", err)
	}

	return outputPath, nil
}
