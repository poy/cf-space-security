package capi_test

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"net/http"
	"testing"

	"github.com/apoydence/cf-space-security/internal/capi"
	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
)

type TV struct {
	*testing.T
	spyDoer *spyDoer
	v       *capi.Validator
}

func TestValidator(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TV {
		spyDoer := newSpyDoer()
		return TV{
			T:       t,
			spyDoer: spyDoer,
			v:       capi.NewValidator("some-id", "http://some.url", spyDoer, log.New(ioutil.Discard, "", 0)),
		}
	})

	o.Spec("returns true for a non-200", func(t TV) {
		t.spyDoer.m["GET:http://some.url/v3/apps/some-id"] = &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewReader(nil)),
		}

		Expect(t, t.v.Validate("some-token")).To(BeTrue())
		Expect(t, t.spyDoer.reqs).To(HaveLen(1))
		Expect(t, t.spyDoer.reqs[0].Header.Get("Authorization")).To(Equal("some-token"))
	})

	o.Spec("returns false for a non-200", func(t TV) {
		t.spyDoer.m["GET:http://some.url/v3/apps/some-id"] = &http.Response{
			StatusCode: 404,
			Body:       ioutil.NopCloser(bytes.NewReader(nil)),
		}

		Expect(t, t.v.Validate("some-token")).To(BeFalse())
	})

	o.Spec("returns false for an empty token", func(t TV) {
		t.spyDoer.m["GET:http://some.url/v3/apps/some-id"] = &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewReader(nil)),
		}

		Expect(t, t.v.Validate("")).To(BeFalse())
	})

	o.Spec("returns false if request fails", func(t TV) {
		t.spyDoer.err = errors.New("some-error")
		t.spyDoer.m["GET:http://some.url/v3/apps/some-id"] = &http.Response{
			StatusCode: 200,
			Body:       ioutil.NopCloser(bytes.NewReader(nil)),
		}

		Expect(t, t.v.Validate("some-token")).To(BeFalse())
	})
}
