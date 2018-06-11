package handlers

import (
	"net/http"
)

type ReverseProxy struct {
	v Validator
	h http.Handler
}

type Validator interface {
	Validate(token string) bool
}

func NewReverseProxy(h http.Handler, v Validator) *ReverseProxy {
	return &ReverseProxy{
		v: v,
		h: h,
	}
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !p.v.Validate(r.Header.Get("Authorization")) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	p.h.ServeHTTP(w, r)
}
