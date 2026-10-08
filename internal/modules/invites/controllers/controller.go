package controllers

import "net/http"

type InviteController interface {
	Create(w http.ResponseWriter, r *http.Request)
	List(w http.ResponseWriter, r *http.Request)
	Revoke(w http.ResponseWriter, r *http.Request)
	Preview(w http.ResponseWriter, r *http.Request)
	Accept(w http.ResponseWriter, r *http.Request)
}
