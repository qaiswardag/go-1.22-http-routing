package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type routeHandler struct{}

func (h *routeHandler) IndexAll(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Index all\n")
}
func (h *routeHandler) ShowByID(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Show by ID\n")
}

func (h *routeHandler) Create(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Create new\n")
}

func (h *routeHandler) UpdateyID(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Update by ID\n")
}
func (h *routeHandler) DeleteByID(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Destroy by ID\n")
}

type wrappedWriter struct {
	http.ResponseWriter
	statusCode int
}

// middleware logic
type Middleware func(http.Handler) http.Handler

func ChainMiddlewares(xs ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		for i := len(xs) - 1; i >= 0; i-- {
			x := xs[i]
			next = x(next)
		}
		return next
	}
}

func (w *wrappedWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
	w.statusCode = statusCode
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := &wrappedWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(wrapped, r)
		log.Println("Middleware Log:", wrapped.statusCode, r.Method, r.URL.Path, time.Since(start))
	})
}

// pass data through routhing context

// example of invalid encodedToken causing an error:
// An encodedToken with invalid characters: "!@#$%^&*()"
// An encodedToken with incorrect padding or length: "YWJjZA"

// example of a valid encodedToken:
// original token: mySecretToken123
// base64 encoded: bXlTZWNyZXRUb2tlbjEyMw==
// curl:
// curl -H "Authorization: Bearer bXlTZWNyZXRUb2tlbjEyMw==" -X POST localhost:6060/v1/listings
// curl -H "Authorization: Bearer bXlTZWNyZXRUb2tlbjEyMw==" localhost:6060/v1/listings

const AuthUserID = "middleware.auth.userID"

func writeUnauthed(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(http.StatusText(http.StatusUnauthorized)))
}

func IsAuthenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")

		// check if header begins with a prefix of Bearer
		if !strings.HasPrefix(authorization, "Bearer ") {
			writeUnauthed(w)
			return
		}

		// pull out the token
		encodedToken := strings.TrimPrefix(authorization, "Bearer ")

		// decode the token from base 64
		token, err := base64.StdEncoding.DecodeString(encodedToken)

		if err != nil {
			writeUnauthed(w)
			log.Println("Invalid encodedToken characters, padding or length:", err)
			return
		}

		// asume a valid base64 token is a valid user id
		userID := string(token)

		ctx := context.WithValue(r.Context(), AuthUserID, userID)
		req := r.WithContext(ctx)

		next.ServeHTTP(w, req)

		// check user
		userID, ok := r.Context().Value(AuthUserID).(string)

		if !ok {
			log.Println("Invalid user ID")
			w.WriteHeader(http.StatusBadRequest)
		}

		log.Println("Creating comment for user:", userID)
	})
}

func main() {
	handler := &routeHandler{}
	v1 := http.NewServeMux()

	// GET
	// listings
	v1.HandleFunc("GET /listing/{id}", handler.ShowByID)
	v1.HandleFunc("GET /listings", handler.IndexAll)
	// votes
	v1.HandleFunc("GET /vote/{id}", handler.ShowByID)
	v1.HandleFunc("GET /votes", handler.IndexAll)

	// PUT
	// listings
	v1.HandleFunc("PUT /listing/{id}", handler.UpdateyID)
	// votes
	v1.HandleFunc("PUT /vote/{id}", handler.UpdateyID)

	// POST
	// listings
	v1.HandleFunc("POST /listing", handler.Create)
	// votes
	v1.HandleFunc("POST /vote", handler.Create)

	// DELETE
	// listings
	v1.HandleFunc("DELETE /listing/{id}", handler.DeleteByID)
	// votes
	v1.HandleFunc("DELETE /vote/{id}", handler.DeleteByID)

	chainMiddlewares := ChainMiddlewares(
		Logging,
		IsAuthenticated,
	)

	wrappedV1 := chainMiddlewares(v1)

	mainRouter := http.NewServeMux()

	mainRouter.Handle("/v1/", http.StripPrefix("/v1", wrappedV1))

	server := http.Server{
		Addr:    ":6060",
		Handler: mainRouter,
	}

	fmt.Println("Server listening")
	if err := server.ListenAndServe(); err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
