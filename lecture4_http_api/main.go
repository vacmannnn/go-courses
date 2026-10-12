package main

import (
	"log"
	"net/http"
	"time"
)

func main() {
	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Println("Books API: http://localhost:8080/books")
	log.Println("Swagger UI: http://localhost:8080/docs/")
	log.Fatal(server.ListenAndServe())
}
