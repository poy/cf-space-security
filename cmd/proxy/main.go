package main

import (
	"crypto/tls"
	"expvar"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/apoydence/cf-space-security/internal/cache"
	"github.com/apoydence/cf-space-security/internal/capi"
	"github.com/apoydence/cf-space-security/internal/handlers"
	"github.com/apoydence/cf-space-security/internal/metrics"
	"github.com/cloudfoundry-incubator/uaago"
	jwt "github.com/dgrijalva/jwt-go"
)

func main() {
	log := log.New(os.Stderr, "[PROXY] ", log.LstdFlags)
	log.Println("starting cf-space-security proxy...")
	defer log.Println("closing cf-space-security proxy...")

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

	tokenAnalyzer := handlers.TokenAnalyzerFunc(func(token string) bool {
		parser := &jwt.Parser{}
		c := jwt.MapClaims{}
		_, _, err := parser.ParseUnverified(token[len("bearer "):], c)
		if err != nil {
			log.Fatalf("failed to parse JWT: %s", err)
		}

		expiresAtF, ok := c["exp"].(float64)
		if !ok {
			log.Fatalf("failed to parse JWT exp")
		}
		expiresAt := time.Unix(int64(expiresAtF), 0)
		return expiresAt.Before(time.Now())
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
		capi.NewRestager(cfg.VcapApplication.ApplicationID, cfg.VcapApplication.CAPIAddr, tokenFetcher, httpClient, log),
		log,
	)

	cacheCreator := func(f func(*http.Request) http.Handler) *cache.Cache {
		return cache.New(cfg.CacheSize, cfg.CacheExpiration, f, m, log)
	}

	ds := domains(cfg, log)
	log.Printf("Proxying for domains: %s", strings.Join(ds, ", "))

	proxy := handlers.NewProxy(
		cfg.SkipSSLValidation,
		ds,
		tokenFetcher,
		cacheCreator,
		tokenAnalyzer,
		log,
	)

	http.HandleFunc("/tokens", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(fmt.Sprintf(`{"access_token":%q}`, proxy.CurrentToken())))
	}))

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

func domains(cfg Config, log *log.Logger) []string {
	var domains []string

	appendDomain := func(addr string) {
		u, err := url.Parse(addr)
		if err != nil {
			log.Fatalf("failed tp parse addr (%s): %s", addr, err)
		}
		domains = append(domains, removeSubdomain(u))
	}

	appendDomain(cfg.VcapApplication.CAPIAddr)

	for _, URI := range cfg.VcapApplication.ApplicationURIs {
		appendDomain(URI)
	}

	return domains
}

func removeSubdomain(u *url.URL) string {
	domains := strings.SplitN(u.Host, ".", 2)
	if len(domains) == 1 {
		return u.Host
	}

	return domains[1]
}

func refreshTokenWatchdog(refToken string, r *capi.Restager, log *log.Logger) {
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
