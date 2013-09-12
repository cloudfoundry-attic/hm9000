package models

import (
	"encoding/base64"
	"encoding/json"
)

type BasicAuthInfo struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

func NewBasicAuthInfoFromJSON(jsonMessage []byte) (BasicAuthInfo, error) {
	authInfo := BasicAuthInfo{}
	err := json.Unmarshal(jsonMessage, &authInfo)
	return authInfo, err
}

func (info BasicAuthInfo) Encode() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(info.User+":"+info.Password))
}
