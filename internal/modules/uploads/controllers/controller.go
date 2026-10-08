package controllers

import "net/http"

type UploadController interface {
	Upload(w http.ResponseWriter, r *http.Request)
	Serve(w http.ResponseWriter, r *http.Request)
}
