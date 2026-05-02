package main

import (
	"net/http"

	"github.com/dmitrymack/go-url-shortener.git/internal/handler"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc(`/`, handler.SetShortUrl)
	mux.HandleFunc(`/{id}`, handler.GetUrlById)

	err := http.ListenAndServe(`:8080`, mux)

	if err != nil {
		panic(err)
	}
}
