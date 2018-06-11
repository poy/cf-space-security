package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apoydence/cf-space-security/internal/handlers"
	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
)

type TR struct {
	*testing.T
	spyValidator *spyValidator
	r            http.Handler
	recorder     *httptest.ResponseRecorder

	spyHandler *spyHandler
}

func TestReverseProxy(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TR {
		spyValidator := newSpyValidator()
		spyHandler := newSpyHandler()
		return TR{
			T:            t,
			spyValidator: spyValidator,
			recorder:     httptest.NewRecorder(),
			spyHandler:   spyHandler,
			r:            handlers.NewReverseProxy(spyHandler, spyValidator),
		}
	})

	o.Spec("it passes request to correct proxy", func(t TR) {
		t.spyValidator.result = true
		req, err := http.NewRequest("GET", "http://some.url", nil)
		Expect(t, err).To(BeNil())
		req.Header.Set("Authorization", "some-token")
		t.r.ServeHTTP(t.recorder, req)

		Expect(t, t.spyValidator.token).To(Equal("some-token"))
		Expect(t, t.recorder.Code).To(Equal(http.StatusOK))
		Expect(t, t.spyHandler.r).To(Not(BeNil()))
		Expect(t, t.spyHandler.w).To(Not(BeNil()))
	})

	o.Spec("it returns a 401 if the validator returns false", func(t TR) {
		req, err := http.NewRequest("GET", "http://some.url", nil)
		Expect(t, err).To(BeNil())
		t.r.ServeHTTP(t.recorder, req)

		Expect(t, t.recorder.Code).To(Equal(http.StatusUnauthorized))

		Expect(t, t.spyHandler.r).To(BeNil())
		Expect(t, t.spyHandler.w).To(BeNil())
	})
}

type spyValidator struct {
	result bool
	token  string
}

func newSpyValidator() *spyValidator {
	return &spyValidator{}
}

func (s *spyValidator) Validate(token string) bool {
	s.token = token
	return s.result
}

type spyHandler struct {
	w http.ResponseWriter
	r *http.Request
}

func newSpyHandler() *spyHandler {
	return &spyHandler{}
}

func (s *spyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.w = w
	s.r = r
}
