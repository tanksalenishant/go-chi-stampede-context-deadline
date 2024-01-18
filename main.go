package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/stampede"
)

var timeA = 5 * time.Second
var timeB = 2 * time.Second
var timeLimit = 3 * time.Second

// Suppose req. A and B share the same resource (DB).

func requestA(ctx context.Context, resultChan chan<- error) {
	select {
	case <-time.After(timeA):
		fmt.Println("Request A completed successfully.")
		resultChan <- nil
	case <-ctx.Done():
		resultChan <- fmt.Errorf("Request A canceled: Time Limit Exceeded: %v", ctx.Err())
	}
}

func requestB(ctx context.Context, resultChan chan<- error) {
	select {
	case <-time.After(timeB):
		fmt.Println("Request B completed successfully.")
		resultChan <- nil
	case <-ctx.Done():
		resultChan <- fmt.Errorf("Request B canceled: Time Limit Exceeded: %v", ctx.Err())
	}
}
func requestAHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeLimit)
	defer cancel()

	resultChan := make(chan error)

	go requestA(ctx, resultChan)

	err := <-resultChan
	if err != nil {
		http.Error(w, fmt.Sprintf("Error from Request A: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, "Request A completed within the timeout.")
}

func requestBHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeLimit)
	defer cancel()

	resultChan := make(chan error)

	go requestB(ctx, resultChan)

	err := <-resultChan
	if err != nil {
		http.Error(w, fmt.Sprintf("Error from Request B: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, "Request B completed within the timeout.")
}

func main() {
	r := chi.NewRouter()
	cached := stampede.Handler(512, 3 * time.Second)
	r.With(cached).Get("/requestA", requestAHandler)

	r.With(cached).Get("/requestB", requestBHandler)

	http.ListenAndServe(":8080", r)
}
