// Various POST request handlers

package server

import (
	"meguca/auth"
	"meguca/config"
	"meguca/imager"
	"meguca/websockets"
	"net/http"
	"strconv"
)

// Create a thread with a finished OP and immediately close it
func createThread(w http.ResponseWriter, r *http.Request) {
	maxSize := config.Get().MaxSize*1024*1024 + jsonLimit
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxSize))

	code, token, err := imager.ParseUpload(r)
	if err != nil {
		imager.LogError(w, r, code, err)
		return
	}

	// Extract file name
	_, header, err := r.FormFile("image")
	if err != nil {
		text500(w, r, err)
		return
	}

	// Map form data to websocket thread creation request
	f := r.Form
	req := websockets.ThreadCreationRequest{
		Subject: f.Get("subject"),
		Board:   f.Get("board"),
		Captcha: auth.Captcha{
			CaptchaID: f.Get("captchaID"),
			Solution:  f.Get("solution"),
		},
		ReplyCreationRequest: websockets.ReplyCreationRequest{
			Image: websockets.ImageRequest{
				Spoiler: f.Get("spoiler") == "on",
				Token:   token,
				Name:    header.Filename,
			},
			SessionCreds: auth.SessionCreds{
				UserID:  f.Get("userID"),
				Session: f.Get("session"),
			},
			Name:     f.Get("name"),
			Password: f.Get("password"),
			Body:     f.Get("body"),
		},
	}

	id, _, _, err := websockets.ConstructThread(req, auth.GetIP(r), true)
	if err != nil {

		// TODO: Not all codes are actually 400. Need to differentiate.

		text400(w, err)
		return
	}

	w.Write([]byte(strconv.FormatUint(id, 10)))
}
