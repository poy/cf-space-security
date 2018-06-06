package handlers_test

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/apoydence/cf-space-security/internal/handlers"
	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
)

type TP struct {
	*testing.T
	spyTokenFetcher *spyTokenFetcher
	server1         *httptest.Server
	server2         *httptest.Server
	headers1        []http.Header
	headers2        []http.Header
	recorder        *httptest.ResponseRecorder
	p               http.Handler
}

func TestProxy(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) *TP {
		tp := &TP{
			T:               t,
			spyTokenFetcher: newSpyTokenFetcher(),
			recorder:        httptest.NewRecorder(),
		}
		server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tp.headers1 = append(tp.headers1, r.Header)
		}))
		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tp.headers2 = append(tp.headers2, r.Header)
		}))

		tp.spyTokenFetcher.token = "some-token"
		tp.p = handlers.NewProxy([]string{server1.URL}, tp.spyTokenFetcher, log.New(ioutil.Discard, "", 0))
		tp.server1 = server1
		tp.server2 = server2
		return tp
	})

	o.Spec("adds authorization header to given domains", func(t *TP) {
		req, err := http.NewRequest("GET", t.server1.URL, nil)
		Expect(t, err).To(BeNil())
		t.p.ServeHTTP(t.recorder, req)

		Expect(t, t.headers1).To(HaveLen(1))
		Expect(t, t.headers1[0].Get("Authorization")).To(Equal("some-token"))
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
