package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/gabrieleiro/rate-limiter/middleware"
)

func hello(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "hello!")
}

func main() {
	requestsPerFrame := flag.Int("requestsPerFrame", 5, "How many requests an IP address can make in a single frame")
	frameDuration := flag.Int("frameDuration", 10, "Duration of a frame in seconds")
	port := flag.Int("port", 8080, "Port to run the server")

	flag.Parse()

	mux := http.NewServeMux()

	mux.HandleFunc("/hello", hello)

	rlm := middleware.NewRateLimiterMiddleware(mux, *requestsPerFrame, time.Duration(*frameDuration) * time.Second, []string{"/metrics"})
	metrics := middleware.NewMetricsMiddleware(rlm)

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, metrics.Report())
	})

	fmt.Printf("listening on port %d\n", *port)
	http.ListenAndServe(fmt.Sprintf(":%d", *port), metrics)
}