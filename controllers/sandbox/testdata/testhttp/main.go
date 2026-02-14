package main

import (
	"log"
	"net/http"
)

func main() {
	log.Fatal(http.ListenAndServe(":80", http.FileServer(http.Dir("/usr/share/nginx/html"))))
}
