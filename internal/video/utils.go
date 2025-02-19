package video

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
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

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	req := &s3.PutObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	presignClient := s3.NewPresignClient(s3Client)

	url, err := presignClient.PresignPutObject(context.TODO(), req, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return url.URL, nil
}
