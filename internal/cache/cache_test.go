package cache_test

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/apoydence/cf-space-security/internal/cache"
	"github.com/apoydence/cf-space-security/internal/metrics"
	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
)

type TC struct {
	*testing.T
	c          *cache.Cache
	spyMetrics *spyMetrics
	spyHandler *spyHandler
	recorder   *httptest.ResponseRecorder
}

func TestCache(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TC {
		spyMetrics := newSpyMetrics()
		spyHandler := newSpyHandler()
		return TC{
			T:          t,
			spyMetrics: spyMetrics,
			spyHandler: spyHandler,
			c:          cache.New(1, time.Second, func(*http.Request) http.Handler { return spyHandler }, spyMetrics, log.New(ioutil.Discard, "", 0)),
			recorder:   httptest.NewRecorder(),
		}
	})

	o.Spec("caches GET requests", func(t TC) {
		req, err := http.NewRequest("GET", "http://some.url", nil)
		Expect(t, err).To(BeNil())

		t.c.ServeHTTP(t.recorder, req)
		Expect(t, t.recorder.Code).To(Equal(15))
		Expect(t, string(t.recorder.Body.Bytes())).To(Equal("http://some.url"))

		t.recorder = httptest.NewRecorder()
		t.c.ServeHTTP(t.recorder, req)
		Expect(t, t.recorder.Code).To(Equal(15))
		Expect(t, string(t.recorder.Body.Bytes())).To(Equal("http://some.url"))

		Expect(t, t.spyHandler.reqs).To(HaveLen(1))
		Expect(t, t.spyHandler.reqs).To(Contain(req))

		Expect(t, t.spyMetrics.GetDelta("CacheMisses")).To(Equal(uint64(1)))
		Expect(t, t.spyMetrics.GetDelta("CacheGetRequests")).To(Equal(uint64(2)))
	})

	o.Spec("it does not cache a non-2XX", func(t TC) {
		t.spyHandler.fail = true
		req, err := http.NewRequest("GET", "http://some.url", nil)
		Expect(t, err).To(BeNil())

		t.c.ServeHTTP(t.recorder, req)
		t.c.ServeHTTP(t.recorder, req)

		Expect(t, t.spyHandler.reqs).To(HaveLen(2))
	})

	o.Spec("accounts for query parameters", func(t TC) {
		req, err := http.NewRequest("GET", "http://some.url?some=value", nil)
		Expect(t, err).To(BeNil())
		t.c.ServeHTTP(t.recorder, req)
		Expect(t, t.recorder.Code).To(Equal(26))
		Expect(t, string(t.recorder.Body.Bytes())).To(Equal("http://some.url?some=value"))

		t.recorder = httptest.NewRecorder()
		req, err = http.NewRequest("GET", "http://some.url", nil)
		Expect(t, err).To(BeNil())
		t.c.ServeHTTP(t.recorder, req)
		Expect(t, t.recorder.Code).To(Equal(15))
		Expect(t, string(t.recorder.Body.Bytes())).To(Equal("http://some.url"))

		Expect(t, t.spyHandler.reqs).To(HaveLen(2))
		Expect(t, t.spyHandler.reqs).To(Contain(req))
		Expect(t, t.spyMetrics.GetDelta("CacheMisses")).To(Equal(uint64(2)))
		Expect(t, t.spyMetrics.GetDelta("CacheGetRequests")).To(Equal(uint64(2)))
	})

	o.Spec("accounts for headers", func(t TC) {
		req, err := http.NewRequest("GET", "http://some.url?some=value", nil)
		Expect(t, err).To(BeNil())
		req.Header.Set("a", "b")
		req.Header.Set("b", "c")

		t.c.ServeHTTP(t.recorder, req)
		Expect(t, t.recorder.Code).To(Equal(26))
		Expect(t, string(t.recorder.Body.Bytes())).To(Equal("http://some.url?some=value"))

		t.recorder = httptest.NewRecorder()
		req, err = http.NewRequest("GET", "http://some.url?some=value", nil)
		Expect(t, err).To(BeNil())
		t.c.ServeHTTP(t.recorder, req)
		Expect(t, t.recorder.Code).To(Equal(26))
		Expect(t, string(t.recorder.Body.Bytes())).To(Equal("http://some.url?some=value"))

		Expect(t, t.spyHandler.reqs).To(HaveLen(2))
		Expect(t, t.spyHandler.reqs).To(Contain(req))
		Expect(t, t.spyMetrics.GetDelta("CacheMisses")).To(Equal(uint64(2)))
		Expect(t, t.spyMetrics.GetDelta("CacheGetRequests")).To(Equal(uint64(2)))
	})

	o.Spec("accounts for header ordering", func(t TC) {
		req, err := http.NewRequest("GET", "http://some.url?some=value", nil)
		Expect(t, err).To(BeNil())
		req.Header.Set("a", "b")
		req.Header.Set("b", "c")

		t.c.ServeHTTP(t.recorder, req)
		Expect(t, t.recorder.Code).To(Equal(26))
		Expect(t, string(t.recorder.Body.Bytes())).To(Equal("http://some.url?some=value"))

		t.recorder = httptest.NewRecorder()
		req, err = http.NewRequest("GET", "http://some.url?some=value", nil)
		Expect(t, err).To(BeNil())
		req.Header.Set("a", "b")
		req.Header.Set("b", "c")

		t.c.ServeHTTP(t.recorder, req)
		Expect(t, t.recorder.Code).To(Equal(26))
		Expect(t, string(t.recorder.Body.Bytes())).To(Equal("http://some.url?some=value"))

		Expect(t, t.spyHandler.reqs).To(HaveLen(1))
		Expect(t, t.spyHandler.reqs).To(Contain(req))
		Expect(t, t.spyMetrics.GetDelta("CacheMisses")).To(Equal(uint64(1)))
		Expect(t, t.spyMetrics.GetDelta("CacheGetRequests")).To(Equal(uint64(2)))
	})

	o.Spec("ignores non-GET requests", func(t TC) {
		req, err := http.NewRequest("POST", "http://some.url", nil)
		Expect(t, err).To(BeNil())

		t.c.ServeHTTP(t.recorder, req)
		t.c.ServeHTTP(t.recorder, req)

		Expect(t, t.spyHandler.reqs).To(HaveLen(2))
		Expect(t, t.spyHandler.reqs).To(Contain(req))
		Expect(t, t.spyMetrics.GetDelta("CacheMisses")).To(Equal(uint64(0)))
		Expect(t, t.spyMetrics.GetDelta("CacheGetRequests")).To(Equal(uint64(0)))
	})

	o.Spec("does not cache if size is 0", func(t TC) {
		t.c = cache.New(0, time.Second, func(*http.Request) http.Handler { return t.spyHandler }, t.spyMetrics, log.New(ioutil.Discard, "", 0))
		req, err := http.NewRequest("GET", "http://some.url", nil)
		Expect(t, err).To(BeNil())

		t.c.ServeHTTP(t.recorder, req)
		t.c.ServeHTTP(t.recorder, req)

		Expect(t, t.spyHandler.reqs).To(HaveLen(2))
		Expect(t, t.spyHandler.reqs).To(Contain(req))
		Expect(t, t.spyMetrics.GetDelta("CacheMisses")).To(Equal(uint64(0)))
		Expect(t, t.spyMetrics.GetDelta("CacheGetRequests")).To(Equal(uint64(0)))
	})
}

type spyHandler struct {
	fail bool
	reqs []*http.Request
}

func newSpyHandler() *spyHandler {
	return &spyHandler{}
}

func (s *spyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.reqs = append(s.reqs, r)
	if s.fail {
		w.WriteHeader(500)
		return
	}

	w.WriteHeader(len(r.URL.String()))
	w.Write([]byte(r.URL.String()))
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
