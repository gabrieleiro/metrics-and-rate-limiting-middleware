package middleware

import (
	"fmt"
	"log"
	"net/http"
	"slices"
	"sync"
	"time"
)

type RateLimiter struct {
	RequestsInCurrentFrame	int
	mu sync.Mutex
}

func (rl *RateLimiter) Limitted(max int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	return rl.RequestsInCurrentFrame >= max
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

func (rl *RateLimiter) ResetTicker(duration time.Duration) {
	c := time.Tick(duration)

	for range c {
		log.Printf("reset\n")
		rl.ResetRequestsInFrame()
	}
}

func NewRateLimiter(frameDuration time.Duration) *RateLimiter {
	rl := &RateLimiter{}
	go rl.ResetTicker(frameDuration)
	return rl
}

type RateLimiterMiddleware struct {
	Mux http.Handler
	MaxRequestsPerFrame int
	FrameDuration time.Duration
	IPs map[string]*RateLimiter
	BypassRoutes []string
}

func (rlm RateLimiterMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// I'm using the IP + port as keys here
	// because it makes it easier to test it locally.
	// In a real-world application I'd strip the port
	addr := r.RemoteAddr
	if _, ok := rlm.IPs[addr]; !ok {
		rlm.IPs[addr] = NewRateLimiter(rlm.FrameDuration)
	}

	if rlm.IPs[addr].Limitted(rlm.MaxRequestsPerFrame) && !slices.Contains(rlm.BypassRoutes, r.URL.Path) {
		fmt.Fprintf(w, "You've been rate limited")
		return
	}

	rlm.IPs[addr].IncrementRequestsInFrame()
	
	rlm.Mux.ServeHTTP(w, r)
}

func NewRateLimiterMiddleware(mux http.Handler, requestsPerFrame int, frameDuration time.Duration, bypassRoutes []string) RateLimiterMiddleware {
	return RateLimiterMiddleware{ 
		IPs: make(map[string]*RateLimiter), 
		Mux: mux,
		MaxRequestsPerFrame: requestsPerFrame, 
		FrameDuration: frameDuration,
		BypassRoutes: bypassRoutes,
	}
}