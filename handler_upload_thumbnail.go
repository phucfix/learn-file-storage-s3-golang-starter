package main

import (
	"fmt"
	"net/http"
	"io"

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

	// Get image data from the form

	// Get the file data and file headers. The key the web browser is using is called "thumbnail"
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close();

	// Get the media type from the form file's Content-Type header
	mediaType := header.Header.Get("Content-Type")

	// Read all the image data into a byte slice using
	imageData, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to read image data", err)
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

	// Save thumbnail to global map
	tb := thumbnail{ data: imageData, mediaType: mediaType}
	videoThumbnails[videoID] = tb

	// Update video meta data
	url := fmt.Sprintf("http://localhost:%d/api/thumbnails/%s", 8091, video.ID.String())
	video.ThumbnailURL = &url

	// Update record in db
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
		return
	}

	// Respond with updated JSON of video's metadata, pass it the updated database.Video struct
	respondWithJSON(w, http.StatusOK, video)
}
