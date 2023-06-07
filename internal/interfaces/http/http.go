package http

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/hugomfc/tt/internal/ratelimiter"
)

type Handler struct {
	rl *ratelimiter.RateLimiter
}

func NewHttpHandler(rl *ratelimiter.RateLimiter) *Handler {
	return &Handler{rl: rl}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Generate the RequestInfo object from the http.Request
	requestInfo := ratelimiter.RequestInfo{
		Method: r.Method,
		IP:     strings.Split(r.RemoteAddr, ":")[0],
		//Headers: r.Header,
		Path: r.URL.String(),
	}

	// Evaluate the rate limiting rules
	allowed, matchedRule, _ := h.rl.Allow(r.Context(), &requestInfo)
	_ = matchedRule
	// If rate limiting is triggered, respond with the appropriate HTTP status code and headers
	log.Println("allowed:", allowed, "matchedRule:", matchedRule, "reqInfo:", requestInfo)
	if !allowed {
		w.Header().Set("X-RateLimit-Exceeded", "true")
		//w.WriteHeader(http.StatusTooManyRequests)
		http.Error(w, "Too many requests", http.StatusTooManyRequests)
		return
	}

	// Else, proceed with the request
	w.Header().Set("X-RateLimit-Exceeded", "false")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(requestInfo)
}
