package main

import (
	"crypto/tls"
	"expvar"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/apoydence/cf-space-security/internal/cache"
	"github.com/apoydence/cf-space-security/internal/handlers"
	"github.com/apoydence/cf-space-security/internal/metrics"
	"github.com/apoydence/cf-space-security/internal/restager"
	"github.com/cloudfoundry-incubator/uaago"
	jwt "github.com/dgrijalva/jwt-go"
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

	m := metrics.New(expvar.NewMap("Proxy"))

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: cfg.SkipSSLValidation,
			},
		},
	}

	go refreshTokenWatchdog(
		cfg.RefreshToken,
		restager.New(cfg.VcapApplication.ApplicationID, cfg.VcapApplication.CAPIAddr, tokenFetcher, httpClient, log),
		log,
	)

	cacheCreator := func(f func(*http.Request) http.Handler) *cache.Cache {
		return cache.New(cfg.CacheSize, cfg.CacheExpiration, f, m, log)
	}

	proxy := handlers.NewProxy(
		cfg.SkipSSLValidation,
		cfg.Domains,
		tokenFetcher,
		cacheCreator,
		log,
	)

	go func() {
		log.Printf("Listening on healthport %d", cfg.HealthPort)
		http.ListenAndServe(
			fmt.Sprintf(":%d", cfg.HealthPort),
			nil,
		)
	}()

	log.Printf("Listening on %d", cfg.Port)
	log.Fatalf("failed to serve: %s",
		http.ListenAndServe(
			fmt.Sprintf(":%d", cfg.Port),
			proxy,
		),
	)
}

func refreshTokenWatchdog(refToken string, r *restager.Restager, log *log.Logger) {
	jwt.Parse(refToken, func(token *jwt.Token) (interface{}, error) {
		claims := token.Claims.(jwt.MapClaims)

		issuedAtF, ok := claims["iat"].(float64)
		if !ok {
			log.Fatalf("failed to parse JWT iat")
		}

		expiresAtF, ok := claims["exp"].(float64)
		if !ok {
			log.Fatalf("failed to parse JWT exp")
		}

		expiresAt := time.Unix(int64(expiresAtF), 0)
		issuedAt := time.Unix(int64(issuedAtF), 0)

		resetTokenIn := issuedAt.Add(expiresAt.Sub(issuedAt) * 90 / 100).Sub(time.Now())
		log.Printf("resetting refresh token in %s", resetTokenIn)

		time.Sleep(resetTokenIn)
		r.SetAndRestage()

		return nil, nil
	})
}
