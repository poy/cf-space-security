package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/poy/cf-space-security/internal/handlers"
	"github.com/poy/onpar"
	. "github.com/poy/onpar/expect"
	. "github.com/poy/onpar/matchers"
)

type TC struct {
	*testing.T
	c http.Handler

	whitelist *spyHandler
	other     *spyHandler

	recorder *httptest.ResponseRecorder
}

func TestController(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TC {
		whitelist := newSpyHandler()
		other := newSpyHandler()

		return TC{
			T:         t,
			whitelist: whitelist,
			other:     other,
			c: handlers.NewController([]string{
				"/great/{args}/still-good",
				"/yay/{key}/{args}",
				"/cool",
			}, whitelist, other),
			recorder: httptest.NewRecorder(),
		}
	})

	o.Spec("routes requests correctly", func(t TC) {
		req, err := http.NewRequest(http.MethodGet, "http://some.url/great/some-arg/still-good", nil)
		Expect(t, err).To(BeNil())
		t.c.ServeHTTP(t.recorder, req)

		Expect(t, t.whitelist.r).To(Not(BeNil()))
		Expect(t, t.other.r).To(BeNil())
		Expect(t, t.whitelist.w).To(Not(BeNil()))

		t.whitelist.r = nil
		t.other.r = nil
		req, err = http.NewRequest(http.MethodGet, "http://some.url/great/some-arg/not-good", nil)
		Expect(t, err).To(BeNil())
		t.c.ServeHTTP(t.recorder, req)

		Expect(t, t.other.r).To(Not(BeNil()))
		Expect(t, t.other.w).To(Not(BeNil()))
		Expect(t, t.whitelist.r).To(BeNil())
	})
}
