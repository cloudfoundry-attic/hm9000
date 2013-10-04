package models

import (
	"encoding/base64"
)

type BasicAuthInfo struct {
	User     string
	Password string
}

func (info BasicAuthInfo) Encode() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(info.User+":"+info.Password))
}
