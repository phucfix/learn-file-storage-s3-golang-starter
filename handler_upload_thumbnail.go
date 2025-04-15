package main

import (
	"fmt"
	"net/http"
	"io"
	"mime"
	"os"

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

	// Hanlder multipart form upload of thumbnail and stores in the maps
	// Bit shift 10 to the left 20 times to get an integer stores proper number of byte
	// Bit shifting is a way to multiply by powers of 2. 10 << 20 is the same as 10 * 1024 * 1024, which is 10MB
	const maxMemory = 10 << 20;
	r.ParseMultipartForm(maxMemory)

	// Get the file data and file headers. The key the web browser is using is called "thumbnail"
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close();

	// Get the media type from the form file's Content-Type header
	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content Type", err)
		return
	}
	if mediaType != "image/jpeg" &&  mediaType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Invalid media type", err)
		return
	}

	// Get video meta data
	video, err := cfg.db.GetVideo(videoID)	
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to get video", err)
		return
	}

	// Authenticated user is not the video owner
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Invalid video owner", err)
		return
	}

	// Save file at specific file path
	assetPath := getAssetPath(videoID, mediaType)
	assetDiskPath := cfg.getAssetDiskPath(assetPath)

	dst, err := os.Create(assetDiskPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create new file", err)
		return
	}
	defer dst.Close()

	// Copy contents to the new file on disk
	if _, err := io.Copy(dst, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to copy file", err)
		return
	}


	// Update video meta data
	tbURL := cfg.getAssetURL(assetPath)
	video.ThumbnailURL = &tbURL

	// Update record in db
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
		return
	}

	// Respond with updated JSON of video's metadata, pass it the updated database.Video struct
	respondWithJSON(w, http.StatusOK, video)
}
