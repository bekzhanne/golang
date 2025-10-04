package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/armanqyzy/golang/practice2/internal/handlers"
	"github.com/armanqyzy/golang/practice2/internal/middleware"
)

func main() {
	mux := http.NewServeMux()

	handler := middleware.APIMiddleware(http.HandlerFunc(handlers.UserHandler))
	mux.Handle("GET /user", handler)
	mux.Handle("POST /user", handler)

	fmt.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
