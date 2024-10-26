package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type RateLimiter struct {
	MaxRequestsPerFrame int
	FrameDuration time.Duration
	RequestsInCurrentFrame	int
	mu sync.Mutex
}

func (rl *RateLimiter) Limitted() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	return rl.RequestsInCurrentFrame >= rl.MaxRequestsPerFrame
}

func (rl *RateLimiter) IncrementRequestsInFrame() {
	rl.mu.Lock()
	rl.RequestsInCurrentFrame++
	log.Printf("incremented to %d", rl.RequestsInCurrentFrame)
	rl.mu.Unlock()
}

func (rl *RateLimiter) ResetRequestsInFrame() {
	rl.mu.Lock()
	rl.RequestsInCurrentFrame = 0
	rl.mu.Unlock()
}

func (rl *RateLimiter) Reseter() {
	c := time.Tick(rl.FrameDuration)

	for range c {
		log.Printf("reset\n")
		rl.ResetRequestsInFrame()
	}
}

func NewRateLimiter(requestsPerFrame int, frameDuration time.Duration) *RateLimiter {
	rl := &RateLimiter{MaxRequestsPerFrame: requestsPerFrame, FrameDuration: frameDuration}
	go rl.Reseter()
	return rl
}

type RateLimiterMiddleware struct {
	mux *http.ServeMux
	RequestsPerFrame int
	FrameDuration time.Duration
	IPs map[string]*RateLimiter
}

func (rlm RateLimiterMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	addr := r.RemoteAddr
	if _, ok := rlm.IPs[addr]; !ok {
		rlm.IPs[addr] = NewRateLimiter(rlm.RequestsPerFrame, rlm.FrameDuration)
	}

	if rlm.IPs[addr].Limitted() {
		fmt.Fprintf(w, "You've been rate limited")
		return
	}

	rlm.IPs[addr].IncrementRequestsInFrame()
	
	rlm.mux.ServeHTTP(w, r)
}

func NewRateLimiterMiddleware(mux *http.ServeMux, requestsPerFrame int, frameDuration time.Duration) RateLimiterMiddleware {
	return RateLimiterMiddleware{ 
		IPs: make(map[string]*RateLimiter), 
		mux: mux, 
		RequestsPerFrame: requestsPerFrame, 
		FrameDuration: frameDuration,
	}
}

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

	rlm := NewRateLimiterMiddleware(mux, *requestsPerFrame, time.Duration(*frameDuration) * time.Second)

	fmt.Printf("listening on port %d\n", *port)
	http.ListenAndServe(fmt.Sprintf(":%d", *port), rlm)
}