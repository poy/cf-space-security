package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/apoydence/cf-space-security/internal/handlers"
	"github.com/cloudfoundry-incubator/uaago"
)

func main() {
	log.Println("starting cf-space-security proxy...")
	defer log.Println("closing cf-space-security proxy...")

	log := log.New(os.Stderr, "", log.LstdFlags)
	log.Println("Starting CF-Space-Security...")
	defer log.Println("Closing CF-Space-Security...")

	cfg := LoadConfig(log)

	uaa, err := uaago.NewClient(cfg.UAAAddr)
	if err != nil {
		log.Fatalf("failed to create UAA client to %s: %s", cfg.UAAAddr, err)
	}

	tokenFetcher := handlers.TokenFetcherFunc(func() string {
		refToken, accessToken, err := uaa.GetRefreshToken(cfg.ClientID, cfg.RefreshToken, cfg.SkipSSLValidation)
		if err != nil {
			log.Fatalf("failed to get refresh token: %s", err)
		}

		cfg.RefreshToken = refToken
		return accessToken
	})

	proxy := handlers.NewProxy(
		cfg.Domains,
		tokenFetcher,
		log,
	)

	log.Fatalf("failed to serve: %s",
		http.ListenAndServe(
			fmt.Sprintf(":%d", cfg.Port),
			proxy,
		),
	)
}
