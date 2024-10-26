package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Request struct {
	time.Duration
	Start, End time.Time
}

type MetricsMiddleware struct {
	mux http.Handler
	mu sync.Mutex
	Requests []Request
}

func (sm *MetricsMiddleware) AddRequest() *Request {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.Requests = append(sm.Requests, Request{ Start: time.Now() })
	return &sm.Requests[len(sm.Requests)-1]
}

type proxyWriter struct {
	http.ResponseWriter
	realWriter http.ResponseWriter
	content    []byte
	statusCode int
	Request *Request
}

func (p *proxyWriter) Write(b []byte) (n int, err error) {
	return p.realWriter.Write(b)
}

func (p *proxyWriter) WriteHeader(statusCode int) {
	p.statusCode = statusCode
}

func (p *proxyWriter) Flush() {
	_, err := p.realWriter.Write(p.content)
	if err != nil {
		log.Printf("could not flush %v", err)
	}

	if p.statusCode != 0 {
		p.realWriter.WriteHeader(p.statusCode)
	}
	
	req := p.Request
	req.End = time.Now()
	req.Duration = req.End.Sub(req.Start)
}

func (sm *MetricsMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	idx := sm.AddRequest()
	pw := &proxyWriter{realWriter: w, Request: idx}
	sm.mux.ServeHTTP(pw, r)
	pw.Flush()
}

func (sm *MetricsMiddleware) Report() string {
	var res strings.Builder
	for i, r := range sm.Requests {
		res.WriteString(fmt.Sprintf("%d: %v\n", i, r.Duration))
	}

	return res.String()
}

func NewMetricsMiddleware(mux http.Handler) MetricsMiddleware {
	return MetricsMiddleware{
		mux: mux,
	}
}
