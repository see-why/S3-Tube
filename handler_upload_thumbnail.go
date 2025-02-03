package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, _, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get thumbnail", err)
		return
	}

	defer file.Close()

	thumbnailBytes, err := io.ReadAll(file)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't read thumbnail", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not your video", nil)
		return
	}

	videoThumbnails[videoID] = thumbnail{
		data:      thumbnailBytes,
		mediaType: r.Header.Get("Content-Type"),
	}

	thumbnailURL := fmt.Sprintf("http://localhost:8091/api/thumbnails/%s", videoID)
	fmt.Print("thumbnailURL: ", thumbnailURL)
	video.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, database.Video{
		ID:           video.ID,
		ThumbnailURL: video.ThumbnailURL,
		VideoURL:     video.VideoURL,
		CreatedAt:    video.CreatedAt,
		UpdatedAt:    video.UpdatedAt,
		CreateVideoParams: database.CreateVideoParams{
			Title:       video.Title,
			Description: video.Description,
			UserID:      video.UserID,
		},
	})
}
