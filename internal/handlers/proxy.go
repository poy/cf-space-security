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

	"github.com/poy/cf-space-security/internal/cache"
)

type Proxy struct {
	f            TokenFetcher
	m            map[string]bool
	c            *cache.Cache
	a            TokenAnalyzer
	proxyCreator func(*http.Request) http.Handler

	mu    sync.RWMutex
	token string
	log   *log.Logger
}

type TokenFetcher interface {
	Token() string
}

type TokenFetcherFunc func() string

func (f TokenFetcherFunc) Token() string {
	return f()
}

type TokenAnalyzer interface {
	Analyze(token string) (expired bool)
}

type TokenAnalyzerFunc func(token string) bool

func (f TokenAnalyzerFunc) Analyze(token string) bool {
	return f(token)
}

type CacheCreator (func(r *http.Request) http.Handler)

func NewProxy(
	skipSSLValidation bool,
	domains []string,
	f TokenFetcher,
	cacheCreator func(func(r *http.Request) http.Handler) *cache.Cache,
	a TokenAnalyzer,
	log *log.Logger,
) *Proxy {
	m := make(map[string]bool)
	for _, domain := range domains {
		m[domain] = true
	}

	p := &Proxy{
		f:     f,
		m:     m,
		a:     a,
		token: f.Token(),
		log:   log,
	}

	p.c = cacheCreator(p.createRevProxy(skipSSLValidation, true))
	p.proxyCreator = p.createRevProxy(skipSSLValidation, false)

	return p
}

func (p *Proxy) CurrentToken() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.token
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Cache-Control") == "no-cache" {
		if p.m[p.removeSubdomain(r.Host)] {
			p.setAuth(r)
		}

		mw := &middleResponseWriter{
			ResponseWriter: w,
		}

		if f, ok := w.(http.Flusher); ok {
			mw.Flusher = f
		}

		p.proxyCreator(r).ServeHTTP(mw, r)

		if mw.statusCode == http.StatusUnauthorized {
			p.clearToken()
		}

		return
	}

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
		for k, v := range recorder.Header() {
			w.Header()[k] = v
		}
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
			p.setAuth(&r)

			p.c.ServeHTTP(mw, &r)
			if mw.statusCode == http.StatusUnauthorized {
				p.clearToken()
				continue
			}

			return
		}

		p.proxyCreator(&r).ServeHTTP(mw, &r)
		return
	}
}

func (p *Proxy) setAuth(r *http.Request) {
	if _, ok := r.Header["Authorization"]; !ok {
		r.Header.Set("Authorization", p.getToken())
	}
}

func (p *Proxy) getToken() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.token == "" {
		p.token = p.f.Token()
	}

	if p.a.Analyze(p.token) {
		p.token = p.f.Token()
	}

	return p.token
}

func (p *Proxy) clearToken() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.token = ""
}

func (p *Proxy) createRevProxy(skipSSLValidation, useHTTPS bool) func(*http.Request) http.Handler {
	return func(r *http.Request) http.Handler {
		u, _ := url.Parse(r.URL.String())
		u.Path = ""
		if u.Scheme == "http" && useHTTPS {
			u.Scheme = "https"
		}

		rp := httputil.NewSingleHostReverseProxy(u)
		rp.ErrorLog = p.log

		rp.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipSSLValidation,
			},
		}

		rp.ModifyResponse = func(resp *http.Response) error {
			if resp.StatusCode == http.StatusFound {
				u, _ := url.Parse(resp.Header.Get("Location"))
				if u != nil && p.m[p.removeSubdomain(u.Host)] {
					resp.Header.Set("Location", strings.Replace(resp.Header.Get("Location"), "https", "http", 1))
				}
			}

			return nil
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
	http.Flusher

	statusCode int
}

func (w *middleResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *middleResponseWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	if err != nil {
		return n, err
	}

	if w.Flusher != nil {
		w.Flush()
	}

	return n, nil
}
