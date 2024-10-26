package middleware

import (
	"fmt"
	"log"
	"net/http"
	"sort"
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
	RequestsPerSecond int
}

func (sm *MetricsMiddleware) AddRequest() *Request {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.Requests = append(sm.Requests, Request{ Start: time.Now() })
	sm.RequestsPerSecond++
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
	var res, requestList strings.Builder
	var totalDuration time.Duration

	requestList.WriteString(fmt.Sprintf("%d Requests per second\n", sm.RequestsPerSecond))
	requestList.WriteString(fmt.Sprintf("%d Requests: \n\n", len(sm.Requests)))
	for i, r := range sm.Requests {
		requestList.WriteString(fmt.Sprintf("%d: %v\n", i+1, r.Duration))
		totalDuration += r.Duration
	}

	sortedByDuration := make([]Request, len(sm.Requests))
	copy(sortedByDuration, sm.Requests)

	sort.Slice(sortedByDuration, func(i, j int) bool {
		return int(sm.Requests[i].Duration) < int(sm.Requests[j].Duration)
	})

	onePercent := len(sm.Requests) / 100
	nineninep := sortedByDuration[onePercent]

	res.WriteString(fmt.Sprintf("Average request duration: %v\n", time.Duration(int(totalDuration) / len(sm.Requests))))
	res.WriteString(fmt.Sprintf("99p: %v\n", nineninep.Duration))
	res.WriteString(requestList.String())

	return res.String()
}

func (ms *MetricsMiddleware) ResetRequestsPerSecond() {
	ms.mu.Lock()
	ms.RequestsPerSecond = 0
	ms.mu.Unlock()
}

func (ms *MetricsMiddleware) RequestPerSecondResetTicker() {
	t := time.Tick(time.Second)

	for range t {
		ms.ResetRequestsPerSecond()
	}
}

func NewMetricsMiddleware(mux http.Handler) *MetricsMiddleware {
	m := &MetricsMiddleware{
		mux: mux,
	}

	go m.RequestPerSecondResetTicker()

	return m
}
