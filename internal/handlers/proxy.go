package handlers

import (
	"bytes"
	"crypto/tls"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/apoydence/cf-space-security/internal/cache"
)

type Proxy struct {
	f            TokenFetcher
	m            map[string]bool
	c            *cache.Cache
	proxyCreator func(*http.Request) http.Handler

	mu    sync.Mutex
	token string
}
type TokenFetcher interface {
	Token() string
}

type TokenFetcherFunc func() string

func (f TokenFetcherFunc) Token() string {
	return f()
}

type CacheCreator (func(r *http.Request) http.Handler)

func NewProxy(
	skipSSLValidation bool,
	domains []string,
	f TokenFetcher,
	cacheCreator func(func(r *http.Request) http.Handler) *cache.Cache,
	log *log.Logger,
) *Proxy {
	m := make(map[string]bool)
	for _, domain := range domains {
		m[domain] = true
	}

	p := &Proxy{
		f:     f,
		m:     m,
		token: f.Token(),
	}

	p.c = cacheCreator(p.createRevProxy(skipSSLValidation, true))
	p.proxyCreator = p.createRevProxy(skipSSLValidation, false)

	return p
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body []byte

	if r.Body != nil {
		var err error
		body, err = ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	var recorder *httptest.ResponseRecorder
	defer func() {
		w.WriteHeader(recorder.Code)
		io.Copy(w, recorder.Body)
	}()

	for i := 0; i < 2; i++ {
		origHeaders := r.Header
		r := *r
		r.Body = ioutil.NopCloser(bytes.NewReader(body))

		// Copy headers
		r.Header = http.Header{}
		for k, v := range origHeaders {
			for _, vv := range v {
				r.Header.Add(k, vv)
			}
		}

		recorder = httptest.NewRecorder()
		mw := &middleResponseWriter{
			ResponseWriter: recorder,
		}

		if p.m[p.removeSubdomain(r.Host)] {
			if _, ok := r.Header["Authorization"]; !ok {
				r.Header.Set("Authorization", p.getToken())
			}

			p.c.ServeHTTP(mw, &r)
			if mw.statusCode == http.StatusUnauthorized {
				p.token = ""
				continue
			}

			return
		}

		p.proxyCreator(&r).ServeHTTP(mw, &r)
		if mw.statusCode == http.StatusUnauthorized {
			p.token = ""
			continue
		}

		return
	}
}

func (p *Proxy) getToken() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.token == "" {
		p.token = p.f.Token()
	}

	return p.token
}

func (p *Proxy) createRevProxy(skipSSLValidation, useHTTPS bool) func(*http.Request) http.Handler {
	return func(r *http.Request) http.Handler {
		u, _ := url.Parse(r.URL.String())
		u.Path = ""
		if u.Scheme == "http" && useHTTPS {
			u.Scheme = "https"
		}

		rp := httputil.NewSingleHostReverseProxy(u)

		rp.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipSSLValidation,
			},
		}
		return rp
	}
}

func (p *Proxy) removeSubdomain(host string) string {
	domains := strings.SplitN(host, ".", 2)
	if len(domains) == 1 {
		return host
	}

	return domains[1]
}

type middleResponseWriter struct {
	http.ResponseWriter

	statusCode int
}

func (w *middleResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}
