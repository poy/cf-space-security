package cache

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sort"
	"time"

	"github.com/apoydence/cf-space-security/internal/metrics"
	"github.com/bluele/gcache"
)

type Cache struct {
	proxyCreator func(*http.Request) http.Handler
	cacheGetReqs func(uint64)

	c   gcache.Cache
	log *log.Logger
}

func New(size int, expire time.Duration, proxyCreator func(r *http.Request) http.Handler, m metrics.Metrics, log *log.Logger) *Cache {
	cacheMiss := m.NewCounter("CacheMisses")

	var c gcache.Cache
	if size > 0 {
		c = gcache.New(size).
			LRU().
			LoaderExpireFunc(func(key interface{}) (interface{}, *time.Duration, error) {
				cacheMiss(1)
				var req request
				if err := json.Unmarshal([]byte(key.(string)), &req); err != nil {
					log.Panic(err)
				}

				r, err := http.NewRequest(http.MethodGet, req.URL, nil)
				if err != nil {
					log.Panic(err)
				}

				for _, header := range req.Headers {
					r.Header[header.Name] = header.Values
				}

				recorder := httptest.NewRecorder()
				proxyCreator(r).ServeHTTP(recorder, r)
				return recorder, &expire, nil
			}).
			Build()
	}

	return &Cache{
		c:            c,
		cacheGetReqs: m.NewCounter("CacheGetRequests"),
		proxyCreator: proxyCreator,
		log:          log,
	}
}

type response struct {
	StatusCode int
	Body       []byte
}

type request struct {
	URL     string
	Headers []struct {
		Name   string
		Values []string
	}
}

func (c *Cache) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if c.c == nil || r.Method != http.MethodGet {
		c.proxyCreator(r).ServeHTTP(w, r)
		return
	}

	var h []struct {
		Name   string
		Values []string
	}
	for k, v := range r.Header {
		sort.Strings(v)
		h = append(h, struct {
			Name   string
			Values []string
		}{
			Name:   k,
			Values: v,
		})
	}
	sort.Sort(headers(h))

	data, err := json.Marshal(request{
		URL:     r.URL.String(),
		Headers: h,
	})
	if err != nil {
		log.Panic(err)
	}

	recorder, err := c.c.Get(string(data))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		c.log.Printf("failed reading from request cache: %s", err)
		return
	}

	rec := recorder.(*httptest.ResponseRecorder)
	for k, v := range rec.Header() {
		w.Header()[k] = v
	}
	w.WriteHeader(rec.Code)
	io.Copy(w, bytes.NewReader(rec.Body.Bytes()))

	c.cacheGetReqs(1)
}

type headers []struct {
	Name   string
	Values []string
}

func (h headers) Len() int {
	return len(h)
}

func (h headers) Swap(i, j int) {
	t := h[i]
	h[i] = h[j]
	h[j] = t
}

func (h headers) Less(i, j int) bool {
	return h[i].Name < h[j].Name
}
