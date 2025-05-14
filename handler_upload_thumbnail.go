package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
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

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	newMediaType := header.Header.Get("Content-Type")

	mediaType, _, err := mime.ParseMediaType(newMediaType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong: parsing MIME", err)
		return
	}

	if mediaType != "image/png" && mediaType != "image/jpeg" {
		respondWithError(w, http.StatusUnsupportedMediaType, "Media type not supported", err)
		return
	}

	fileExtension := strings.TrimPrefix(newMediaType, "image/")

	fileName := filepath.Join(cfg.assetsRoot, fmt.Sprintf("%v.%v", videoIDString, fileExtension))

	imgFile, err := os.Create(fileName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong: creating file", err)
		return
	}

	_, err = io.Copy(imgFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong: copying file", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong: fetching video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User does not own the video", err)
		return
	}

	newThumbnailURL := fmt.Sprintf("http://localhost:%v/assets/%v.%v", cfg.port, videoIDString, fileExtension)

	video.ThumbnailURL = &(newThumbnailURL)

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong: updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
