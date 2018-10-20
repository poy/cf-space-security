package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/poy/cf-space-security/internal/capi"
	"github.com/poy/cf-space-security/internal/handlers"
)

func main() {
	log := log.New(os.Stderr, "[REVERSE-PROXY] ", log.LstdFlags)
	log.Println("starting cf-space-security reverse proxy...")
	defer log.Println("closing cf-space-security reverse proxy...")

	cfg := LoadConfig(log)

	validator := capi.NewValidator(
		cfg.VcapApplication.ApplicationID,
		strings.Replace(cfg.VcapApplication.CAPIAddr, "https", "http", 1),
		http.DefaultClient,
		log,
	)

	u, err := url.Parse(fmt.Sprintf("http://localhost:%d", cfg.BackendPort))
	if err != nil {
		log.Fatalf("failed to parse backend addr: %s", err)
	}

	rp := httputil.NewSingleHostReverseProxy(u)
	rp.ErrorLog = log
	revProxy := handlers.NewReverseProxy(rp, validator)
	controller := handlers.NewController(
		cfg.OpenEndpoints,
		httputil.NewSingleHostReverseProxy(u),
		revProxy,
	)

	log.Printf("Listening on %d", cfg.Port)
	log.Fatalf("failed to serve: %s",
		http.ListenAndServe(
			fmt.Sprintf(":%d", cfg.Port),
			controller,
		),
	)
}
