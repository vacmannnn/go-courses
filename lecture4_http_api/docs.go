package main

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var specification []byte

//go:embed swagger.html
var swaggerHTML []byte

func serveSpec(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(specification)
}

func serveDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(swaggerHTML)
}
