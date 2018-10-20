package metrics_test

import (
	"expvar"
	"testing"

	"github.com/poy/cf-space-security/internal/metrics"
	"github.com/poy/onpar"
	. "github.com/poy/onpar/expect"
	. "github.com/poy/onpar/matchers"
)

type TM struct {
	*testing.T
	spyMap *spyMap
	m      metrics.Metrics
}

func TestMetrics(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TM {
		spyMap := newSpyMap()
		return TM{
			T:      t,
			spyMap: spyMap,
			m:      metrics.New(spyMap),
		}
	})

	o.Spec("publishes the total of a counter", func(t TM) {
		c := t.m.NewCounter("some-counter")
		c(99)
		c(101)

		Expect(t, t.spyMap.getValue("some-counter")).To(Equal(float64(200)))
	})

	o.Spec("publishes the value of a gauge", func(t TM) {
		c := t.m.NewGauge("some-gauge")
		c(99.9)
		c(101.1)

		Expect(t, t.spyMap.getValue("some-gauge")).To(Equal(101.1))
	})

	o.Spec("deals with a nil map", func(t TM) {
		t.m = metrics.New(nil)
		Expect(t, func() { t.m.NewGauge("some-gauge") }).To(Not(Panic()))
		Expect(t, func() { t.m.NewCounter("some-counter") }).To(Not(Panic()))
	})
}

type spyMap struct {
	m map[string]expvar.Var
}

func newSpyMap() *spyMap {
	return &spyMap{
		m: make(map[string]expvar.Var),
	}
}

func (s *spyMap) Add(key string, delta int64) {
	s.m[key] = expvar.NewInt(key)
}

func (s *spyMap) AddFloat(key string, delta float64) {
	s.m[key] = expvar.NewFloat(key)
}

func (s *spyMap) Get(key string) expvar.Var {
	return s.m[key]
}

func (s *spyMap) getValue(key string) float64 {
	switch x := s.m[key].(type) {
	case *expvar.Float:
		return x.Value()
	case *expvar.Int:
		return float64(x.Value())
	default:
		panic("Unknown type...")
	}
}
