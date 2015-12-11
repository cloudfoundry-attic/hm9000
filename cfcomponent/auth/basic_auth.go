package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Auth interface {
	Wrap(wrappedHandler func(http.ResponseWriter, *http.Request)) func(w http.ResponseWriter, req *http.Request)
}

type BasicAuth struct {
	realm       string
	credentials []string
}

const (
	username = iota
	password
)

func NewBasicAuth(realm string, credentials []string) Auth {
	return BasicAuth{realm: realm, credentials: credentials}
}

func (auth BasicAuth) Wrap(wrappedHandler func(http.ResponseWriter, *http.Request)) func(w http.ResponseWriter, req *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		if !auth.validCredentials(req) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("WWW-Authenticate", `Basic realm="`+auth.realm+`"`)
			fmt.Fprintf(w, "%d Unauthorized", http.StatusUnauthorized)
			return
		}

		wrappedHandler(w, req)
	}
}

func (auth BasicAuth) validCredentials(req *http.Request) bool {
	creds, err := extractCredentials(req)
	if err != nil {
		return false
	}
	return creds[username] == auth.credentials[username] && creds[password] == auth.credentials[password]
}

func extractCredentials(req *http.Request) ([]string, error) {
	authorizationHeader := req.Header.Get("Authorization")
	basicAuthParts := strings.Split(authorizationHeader, " ")
	if len(basicAuthParts) != 2 || basicAuthParts[0] != "Basic" {
		return nil, errors.New(fmt.Sprintf("Malformed authorization header: %s", authorizationHeader))
	}

	decodedUserAndPassword, err := base64.StdEncoding.DecodeString(basicAuthParts[1])
	if err != nil {
		return nil, err
	}

	userAndPassword := strings.Split(string(decodedUserAndPassword), ":")
	if len(userAndPassword) != 2 {
		return nil, errors.New(fmt.Sprintf("Malformed authorization header: %s", authorizationHeader))
	}

	return userAndPassword, nil
}
