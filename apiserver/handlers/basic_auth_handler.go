package handlers

import (
	"net/http"

	"github.com/goji/httpauth"
)

func BasicAuthWrap(handler http.Handler, username, password string) http.Handler {
	opts := httpauth.AuthOptions{
		Realm:               "API Authentication",
		User:                username,
		Password:            password,
		UnauthorizedHandler: http.HandlerFunc(unauthorized),
	}
	return httpauth.BasicAuth(opts)(handler)
}

func unauthorized(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
}
