package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

type Stream struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type FfprobeOutput struct {
	Streams []Stream `json:"streams"`
}

// getVideoAspectRatio retrieves the aspect ratio of a video file
func getVideoAspectRatio(filePath string) (string, error) {
	// Prepare the ffprobe command
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	// Set up a buffer to capture the output
	var out bytes.Buffer
	cmd.Stdout = &out

	// Run the command
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to run ffprobe: %w", err)
	}

	// Unmarshal the JSON output
	var ffprobeOutput FfprobeOutput
	if err := json.Unmarshal(out.Bytes(), &ffprobeOutput); err != nil {
		return "", fmt.Errorf("failed to unmarshal ffprobe output: %w", err)
	}

	// Check if we have any streams
	if len(ffprobeOutput.Streams) == 0 {
		return "", fmt.Errorf("no streams found in video file")
	}

	// Get the width and height of the first stream
	width := ffprobeOutput.Streams[0].Width
	height := ffprobeOutput.Streams[0].Height

	// Calculate the aspect ratio
	if width == height {
		return "1:1", nil // Square aspect ratio
	} else if width > height {
		if width*9 == height*16 {
			return "16:9", nil
		}
		return "other", nil
	} else {
		if height*9 == width*16 {
			return "9:16", nil
		}
		return "other", nil
	}
}

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	uploadLimt := 1 << 30
	reader := http.MaxBytesReader(w, r.Body, int64(uploadLimt))
	defer r.Body.Close()
	defer reader.Close()

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading VIDEO", videoID, "by user", userID)

	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return
	}

	if videoData.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized", nil)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get video", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
		return
	}

	osFile, err := os.CreateTemp("", "*-tubely-video-mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save file", err)
		return
	}

	defer os.Remove(osFile.Name())
	defer osFile.Close()

	io.Copy(osFile, file)
	_, err = osFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't seek file", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(osFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get aspect ratio", err)
		return
	}

	switch aspectRatio {
	case "16:9":
		aspectRatio = "landscape"
	case "4:3":
		aspectRatio = "portrait"
	default:
		aspectRatio = "other"
	}

	bytes := make([]byte, 32)
	_, err = rand.Read(bytes)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to save thumbnail", err)
		return
	}

	fileName := base64.RawURLEncoding.EncodeToString(bytes)

	_, err = cfg.s3Client.PutObject(
		context.TODO(),
		&s3.PutObjectInput{
			Bucket:      aws.String(cfg.s3Bucket),
			Key:         aws.String(fmt.Sprintf("%s%s.mp4", aspectRatio, fileName)),
			Body:        osFile,
			ContentType: aws.String(mediaType),
		})

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't upload video", err)
		return
	}

	newUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s.mp4", cfg.s3Bucket, cfg.s3Region, fileName)
	videoData.VideoURL = &newUrl
	err = cfg.db.UpdateVideo(videoData)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoData)
}
