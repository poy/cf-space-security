package handlers_test

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/apoydence/cf-space-security/internal/cache"
	"github.com/apoydence/cf-space-security/internal/handlers"
	"github.com/apoydence/cf-space-security/internal/metrics"
	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
)

type TP struct {
	*testing.T
	spyTokenFetcher  *spyTokenFetcher
	spyTokenAnalyzer *spyTokenAnalyzer

	server1   *httptest.Server
	server2   *httptest.Server
	return401 bool

	headers1 []http.Header
	headers2 []http.Header
	recorder *httptest.ResponseRecorder
	p        *handlers.Proxy
}

func TestProxy(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) *TP {
		tp := &TP{
			T:                t,
			spyTokenFetcher:  newSpyTokenFetcher(),
			spyTokenAnalyzer: newSpyTokenAnalyzler(),
			recorder:         httptest.NewRecorder(),
		}

		tp.server1 = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tp.headers1 = append(tp.headers1, r.Header)

			if tp.return401 {
				w.WriteHeader(401)
			}
		}))

		tp.server2 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tp.headers2 = append(tp.headers2, r.Header)
		}))

		tp.spyTokenFetcher.token = "some-token"
		tp.p = handlers.NewProxy(
			true,
			[]string{tp.server1.URL[7:]},
			tp.spyTokenFetcher,
			func(f func(r *http.Request) http.Handler) *cache.Cache {
				return cache.New(1, time.Minute, f, newSpyMetrics(), log.New(ioutil.Discard, "", 0))
			},
			tp.spyTokenAnalyzer,
			log.New(os.Stderr, "", 0),
		)
		return tp
	})

	o.Spec("adds authorization header to given domains", func(t *TP) {
		req, err := http.NewRequest("GET", t.server1.URL, nil)
		Expect(t, err).To(BeNil())
		req.Host = "api." + t.server1.URL[7:]
		t.p.ServeHTTP(t.recorder, req)

		Expect(t, t.headers1).To(HaveLen(1))
		Expect(t, t.headers1[0].Get("Authorization")).To(Equal("some-token"))
	})

	o.Spec("does not overwrite authorization header", func(t *TP) {
		req, err := http.NewRequest("GET", t.server1.URL, nil)
		Expect(t, err).To(BeNil())
		req.Header.Set("Authorization", "my-token")

		req.Host = "api." + t.server1.URL[7:]
		t.p.ServeHTTP(t.recorder, req)

		Expect(t, t.headers1).To(HaveLen(1))
		Expect(t, t.headers1[0].Get("Authorization")).To(Equal("my-token"))
	})

	o.Spec("caches requests", func(t *TP) {
		req, err := http.NewRequest("GET", t.server1.URL, nil)
		Expect(t, err).To(BeNil())
		req.Host = "api." + t.server1.URL[7:]
		t.p.ServeHTTP(t.recorder, req)
		t.p.ServeHTTP(t.recorder, req)

		Expect(t, t.headers1).To(HaveLen(1))
		Expect(t, t.headers1[0].Get("Authorization")).To(Equal("some-token"))
		Expect(t, t.spyTokenFetcher.called).To(Equal(1))
	})

	o.Spec("skips cache for Cache-Control no-cache", func(t *TP) {
		req, err := http.NewRequest("GET", t.server1.URL, nil)
		Expect(t, err).To(BeNil())
		req.Host = "api." + t.server1.URL[7:]
		req.Header.Set("Cache-Control", "no-cache")
		t.p.ServeHTTP(t.recorder, req)
		delete(req.Header, "Authorization")
		t.p.ServeHTTP(t.recorder, req)

		Expect(t, t.headers1).To(HaveLen(2))
		Expect(t, t.headers1[0].Get("Authorization")).To(Equal("some-token"))
		Expect(t, t.spyTokenFetcher.called).To(Equal(1))
	})

	o.Spec("requests new token on 401 with Cache-Control no-cache", func(t *TP) {
		t.return401 = true
		req, err := http.NewRequest("GET", t.server1.URL, nil)
		Expect(t, err).To(BeNil())
		req.Host = "api." + t.server1.URL[7:]
		req.Header.Set("Cache-Control", "no-cache")
		t.p.ServeHTTP(t.recorder, req)
		delete(req.Header, "Authorization")
		t.p.ServeHTTP(t.recorder, req)

		Expect(t, t.headers1).To(HaveLen(2))
		Expect(t, t.headers1[0].Get("Authorization")).To(Equal("some-token"))
		Expect(t, t.spyTokenFetcher.called).To(Equal(2))
	})

	o.Spec("requests new token on 401", func(t *TP) {
		t.return401 = true
		req, err := http.NewRequest("GET", t.server1.URL, nil)
		Expect(t, err).To(BeNil())
		req.Host = "api." + t.server1.URL[7:]
		t.p.ServeHTTP(t.recorder, req)

		Expect(t, t.headers1).To(HaveLen(2))
		Expect(t, t.headers1[0].Get("Authorization")).To(Equal("some-token"))
		Expect(t, t.spyTokenFetcher.called).To(Equal(2))
	})

	o.Spec("requests new token if it expires", func(t *TP) {
		t.spyTokenAnalyzer.isExpired = true

		req, err := http.NewRequest("GET", t.server1.URL, nil)
		Expect(t, err).To(BeNil())
		req.Host = "api." + t.server1.URL[7:]

		t.p.ServeHTTP(t.recorder, req)
		t.p.ServeHTTP(t.recorder, req)

		Expect(t, t.spyTokenFetcher.called).To(BeAbove(1))
	})

	o.Spec("does not add authorization header to non-given domains", func(t *TP) {
		req, err := http.NewRequest("GET", t.server2.URL, nil)
		Expect(t, err).To(BeNil())
		t.p.ServeHTTP(t.recorder, req)

		Expect(t, t.headers2).To(HaveLen(1))
		Expect(t, t.headers2[0].Get("Authorization")).To(Not(Equal("some-token")))
	})

	o.Spec("it survives the race detector", func(t *TP) {
		go func() {
			for i := 0; i < 100; i++ {
				req, err := http.NewRequest("GET", t.server1.URL, nil)
				Expect(t, err).To(BeNil())
				t.p.ServeHTTP(httptest.NewRecorder(), req)
			}
		}()

		for i := 0; i < 100; i++ {
			req, err := http.NewRequest("GET", t.server2.URL, nil)
			Expect(t, err).To(BeNil())
			t.p.ServeHTTP(httptest.NewRecorder(), req)
		}
	})

	o.Spec("it returns the current token", func(t *TP) {
		Expect(t, t.p.CurrentToken()).To(Equal("some-token"))
	})
}

type spyTokenFetcher struct {
	mu     sync.Mutex
	called int
	token  string
}

func newSpyTokenFetcher() *spyTokenFetcher {
	return &spyTokenFetcher{}
}

func (s *spyTokenFetcher) Token() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.called++
	return s.token
}

type spyMetrics struct {
	metrics.Metrics

	mu sync.Mutex
	m  map[string]uint64
}

func newSpyMetrics() *spyMetrics {
	return &spyMetrics{
		m: make(map[string]uint64),
	}
}

func (s *spyMetrics) NewCounter(name string) func(uint64) {
	return func(delta uint64) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.m[name] += delta
	}
}

func (s *spyMetrics) GetDelta(name string) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[name]
}

type spyTokenAnalyzer struct {
	token string

	isExpired bool
}

func newSpyTokenAnalyzler() *spyTokenAnalyzer {
	return &spyTokenAnalyzer{}
}

func (s *spyTokenAnalyzer) Analyze(token string) bool {
	s.token = token

	return s.isExpired
}
