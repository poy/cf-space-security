package handlers

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

type Proxy struct {
	f TokenFetcher
	m map[string]*httputil.ReverseProxy
}

type TokenFetcher interface {
	Token() string
}

type TokenFetcherFunc func() string

func (f TokenFetcherFunc) Token() string {
	return f()
}

func NewProxy(domains []string, f TokenFetcher, log *log.Logger) *Proxy {
	m := make(map[string]*httputil.ReverseProxy)
	for _, domain := range domains {
		u, err := url.Parse(domain)
		if err != nil {
			log.Fatal(err)
		}
		m[u.Host] = httputil.NewSingleHostReverseProxy(u)
	}

	return &Proxy{
		f: f,
		m: m,
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	proxy, ok := p.m[r.Host]
	if ok {
		r.Header.Set("Authorization", p.f.Token())
		proxy.ServeHTTP(w, r)
		return
	}

	httputil.NewSingleHostReverseProxy(r.URL).ServeHTTP(w, r)
}
