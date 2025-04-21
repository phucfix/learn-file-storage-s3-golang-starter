package main

import (
	"net/http"
	"mime"
	"os"
	"io"
	"context"
	
	"github.com/google/uuid"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	// Validate user authentication
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
	}

	// Extract video ID from the URL parameters and parse it as a UUID
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	// Valid video owner
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't find video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Not authorized to update this video", nil)
		return
	}

	// Limit request body
	const maxMemory = 1 << 30 // 1Gb
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)
	defer r.Body.Close()

	// Parse uploaded video file from the form data
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	// Validate the uploaded file to ensure it's an MP4 video
	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", nil)
		return
	}

	// Save the uploaded file to temp file on disk
	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't temporary save file", err)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close() // defer is LIFO, so it will close before the remove

	if _, err := io.Copy(tempFile, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error saving file", err)
		return
	}

	// Reset the tempFile's file pointer to the beginning, allow to read the file again from beginning
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error reset temp files pointer", err)
		return
	}

	// Put the object into S3
	fileKey := getAssetPath(mediaType)
	cfg.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: 	 aws.String(cfg.s3Bucket),
		Key: 		 aws.String(fileKey),
		Body: 		 tempFile,
		ContentType: aws.String(mediaType),
	})

	// update the VideoURL of the video record in the database with the S3 bucket and key
	// S3 URLs are in the format https://<bucket-name>.s3.<region>.amazonaws.com/<key>
	url := cfg.getObjectURL(fileKey)
	video.VideoURL = &url
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
