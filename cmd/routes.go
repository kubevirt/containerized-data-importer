package main

import "net/http"

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type Routes []Route

var routes = Routes{
	Route{
		"Index",
		"GET",
		"/",
		Index,
	},
	Route{
		"Pic",
		"GET",
		"/pic",
		ShowPic,
	},
	Route{
		"GetFile",
		"GET",
		"/getBucket/{bucketName}/{fileName}",
		GetFile,
	},
	Route{
		"GetAllFile",
		"GET",
		"/getBucket/{bucketName}",
		GetAllFile,
	},
	Route{
		"PutFile",
		"POST",
		"/putFile/{bucketName}/{fileName}",
		PutFile,
	},
}
