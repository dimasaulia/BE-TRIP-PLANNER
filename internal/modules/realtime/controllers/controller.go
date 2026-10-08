package controllers

import "net/http"

type RealtimeController interface {
	Connect(w http.ResponseWriter, r *http.Request)
}
