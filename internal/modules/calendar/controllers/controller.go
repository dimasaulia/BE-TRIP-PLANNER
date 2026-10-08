package controllers

import "net/http"

type CalendarController interface {
	Connect(w http.ResponseWriter, r *http.Request)
	Callback(w http.ResponseWriter, r *http.Request)
	Disconnect(w http.ResponseWriter, r *http.Request)
	EnableSync(w http.ResponseWriter, r *http.Request)
	DisableSync(w http.ResponseWriter, r *http.Request)
	Resync(w http.ResponseWriter, r *http.Request)
}
