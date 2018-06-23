package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
)

type Controller struct {
	r http.Handler
}

func NewController(
	whiteListPaths []string,
	whiteList http.Handler,
	other http.Handler,
) *Controller {
	mux := mux.NewRouter()

	for _, path := range whiteListPaths {
		mux.Handle(path, whiteList)
	}

	// catch-all
	mux.PathPrefix("/").Handler(other)

	return &Controller{
		r: mux,
	}
}

func (c *Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.r.ServeHTTP(w, r)
}
