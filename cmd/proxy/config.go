package main

import (
	"encoding/json"
	"log"
	"strings"

	"code.cloudfoundry.org/go-envstruct"
)

type Config struct {
	Port    int      `env:"PORT, required, report"`
	Domains []string `env:"DOMAINS, required, report"`

	VcapApplication VcapApplication `env:"VCAP_APPLICATION, required"`

	ClientID     string `env:"CLIENT_ID, required"`
	RefreshToken string `env:"REFRESH_TOKEN, required"`

	SkipSSLValidation bool `env:"SKIP_SSL_VALIDATION, report"`

	// Figured out via VcapApplication
	UAAAddr string
}

type VcapApplication struct {
	CAPIAddr        string   `json:"cf_api"`
	ApplicationID   string   `json:"application_id"`
	ApplicationURIs []string `json:"application_uris"`
}

func (a *VcapApplication) UnmarshalEnv(data string) error {
	return json.Unmarshal([]byte(data), a)
}

func LoadConfig(log *log.Logger) Config {
	cfg := Config{}
	if err := envstruct.Load(&cfg); err != nil {
		log.Fatal(err)
	}

	cfg.UAAAddr = strings.Replace(cfg.VcapApplication.CAPIAddr, "api", "uaa", 1)

	envstruct.WriteReport(&cfg)

	return cfg
}
