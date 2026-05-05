package main

import (
	"net/http"

	"github.com/dmitrymack/go-url-shortener.git/internal/handler"
	"github.com/go-chi/chi/v5"
)

func main() {

	r := chi.NewRouter()
	r.Post("/", handler.SetShortUrl)
	r.Get("/{id}", handler.GetUrlById)

	err := http.ListenAndServe(`:8080`, r)

	if err != nil {
		panic(err)
	}
}
