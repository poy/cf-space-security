package restager_test

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"testing"

	"github.com/apoydence/cf-space-security/internal/restager"
	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
)

type TR struct {
	*testing.T
	spyTokenFetcher *spyTokenFetcher
	spyDoer         *spyDoer
	r               *restager.Restager
}

func TestRestager(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TR {
		spyTokenFetcher := newSpyTokenFetcher()
		spyDoer := newSpyDoer()
		return TR{
			T:               t,
			spyTokenFetcher: spyTokenFetcher,
			spyDoer:         spyDoer,
			r:               restager.New("some-id", "https://some.com", spyTokenFetcher, spyDoer, log.New(ioutil.Discard, "", 0)),
		}
	})

	o.Spec("it sets the environment variables", func(t TR) {
		t.spyTokenFetcher.token = "some-token"
		t.spyDoer.m["PUT:https://some.com/v2/apps/some-id"] = &http.Response{
			StatusCode: 201,
			Body:       ioutil.NopCloser(bytes.NewReader(nil)),
		}

		t.spyDoer.m["POST:https://some.com/v2/apps/some-id/restage"] = &http.Response{
			StatusCode: 201,
			Body:       ioutil.NopCloser(bytes.NewReader(nil)),
		}

		t.r.SetAndRestage()

		Expect(t, t.spyDoer.reqs).To(HaveLen(2))
		Expect(t, t.spyDoer.reqs[0].Method).To(Equal("PUT"))
		Expect(t, t.spyDoer.reqs[0].URL.Scheme).To(Equal("https"))
		Expect(t, t.spyDoer.reqs[0].URL.Host).To(Equal("some.com"))
		Expect(t, t.spyDoer.reqs[0].URL.Path).To(Equal("/v2/apps/some-id"))
		Expect(t, t.spyDoer.bodies[0]).To(MatchJSON(`{"environment_json":{"REFRESH_TOKEN":"some-token"}}`))

		Expect(t, t.spyDoer.reqs[1].Method).To(Equal("POST"))
		Expect(t, t.spyDoer.reqs[1].URL.Scheme).To(Equal("https"))
		Expect(t, t.spyDoer.reqs[1].URL.Host).To(Equal("some.com"))
		Expect(t, t.spyDoer.reqs[1].URL.Path).To(Equal("/v2/apps/some-id/restage"))
		Expect(t, t.spyDoer.bodies[1]).To(HaveLen(0))
	})
}

type spyTokenFetcher struct {
	called int
	token  string
}

func newSpyTokenFetcher() *spyTokenFetcher {
	return &spyTokenFetcher{}
}

func (s *spyTokenFetcher) Token() string {
	s.called++
	return s.token
}

type spyDoer struct {
	m      map[string]*http.Response
	reqs   []*http.Request
	bodies []string
	err    error
}

func newSpyDoer() *spyDoer {
	return &spyDoer{
		m: make(map[string]*http.Response),
	}
}

func (s *spyDoer) Do(r *http.Request) (*http.Response, error) {
	s.reqs = append(s.reqs, r)

	if r.Body == nil {
		r.Body = ioutil.NopCloser(bytes.NewReader(nil))
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	s.bodies = append(s.bodies, string(data))

	if resp, ok := s.m[r.Method+":"+r.URL.String()]; ok {
		return resp, s.err
	}

	return &http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewReader(nil)),
	}, s.err
}
