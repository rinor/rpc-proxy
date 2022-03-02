package main

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type limiters struct {
	noLimitIPs map[string]struct{} // concurrent read safe after init.
	visitors   map[string]*rate.Limiter
	sync.RWMutex
}

func (ls *limiters) tryAddVisitor(ip string, requestsPerMinuteLimit int) (*rate.Limiter, bool) {
	ls.Lock()
	defer ls.Unlock()
	limiter, exists := ls.visitors[ip]
	if exists {
		return limiter, false
	}
	limit := rate.Every(time.Minute / time.Duration(requestsPerMinuteLimit))
	limiter = rate.NewLimiter(limit, requestsPerMinuteLimit/10)
	ls.visitors[ip] = limiter
	return limiter, true
}

func (ls *limiters) getVisitor(ip string, requestsPerMinuteLimit int) (*rate.Limiter, bool) {
	ls.RLock()
	limiter, exists := ls.visitors[ip]
	ls.RUnlock()
	if !exists {
		return ls.tryAddVisitor(ip, requestsPerMinuteLimit)
	}
	return limiter, false
}

func (ls *limiters) AllowVisitor(ip string, requestsPerMinuteLimit int) (allowed, added bool) {
	if _, ok := ls.noLimitIPs[ip]; ok {
		return true, false
	}
	limiter, added := ls.getVisitor(ip, requestsPerMinuteLimit)
	return limiter.Allow(), added
}
