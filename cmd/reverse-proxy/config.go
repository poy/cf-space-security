package main

import (
	"encoding/json"
	"log"

	"code.cloudfoundry.org/go-envstruct"
)

type Config struct {
	Port        int `env:"PORT, required, report"`
	BackendPort int `env:"BACKEND_PORT, required, report"`

	// Whitelist of endpoints that auth is not required.
	OpenEndpoints []string `env:"OPEN_ENDPOINTS, report"`

	VcapApplication VcapApplication `env:"VCAP_APPLICATION, required"`
}

type VcapApplication struct {
	CAPIAddr      string `json:"cf_api"`
	ApplicationID string `json:"application_id"`
}

func (a *VcapApplication) UnmarshalEnv(data string) error {
	return json.Unmarshal([]byte(data), a)
}

func LoadConfig(log *log.Logger) Config {
	cfg := Config{}
	if err := envstruct.Load(&cfg); err != nil {
		log.Fatal(err)
	}

	envstruct.WriteReport(&cfg)

	return cfg
}
