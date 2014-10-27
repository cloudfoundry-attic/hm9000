package apiserver

import "github.com/tedsuo/rata"

var Routes = rata.Routes{
	{Method: "POST", Name: "bulk_app_state", Path: "/bulk_app_state"},
}
