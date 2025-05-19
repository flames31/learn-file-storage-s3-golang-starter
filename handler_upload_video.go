package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const UPLOAD_LIMIT = 1 << 30

	r.Body = http.MaxBytesReader(w, r.Body, UPLOAD_LIMIT)

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

	fmt.Println("uploading video", videoID, "by user", userID)

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong: fetching video", err)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User does not own the video", err)
		return
	}

	file, header, err := r.FormFile("video")
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

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusUnsupportedMediaType, "Media type not supported", err)
		return
	}
	fileExtension := strings.TrimPrefix(newMediaType, "video/")
	bytes := make([]byte, 32)
	rand.Read(bytes)
	randString := base64.RawURLEncoding.EncodeToString(bytes)
	fileName := fmt.Sprintf("%v.%v", randString, fileExtension)

	tempFile, err := os.CreateTemp("", "tubely-upload")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Something went wrong: creating temp file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	io.Copy(tempFile, file)
	tempFile.Seek(0, io.SeekStart)

	cfg.s3Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileName,
		Body:        tempFile,
		ContentType: &newMediaType,
	})

	videoUrl := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", cfg.s3Bucket, cfg.s3Region, fileName)

	video.VideoURL = &videoUrl

	cfg.db.UpdateVideo(video)

	respondWithJSON(w, http.StatusOK, video)
}
