package capi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type Restager struct {
	f       TokenFetcher
	d       Doer
	log     *log.Logger
	apiAddr string
	appID   string
}

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type TokenFetcher interface {
	Token() string
}

func NewRestager(appID, apiAddr string, f TokenFetcher, d Doer, log *log.Logger) *Restager {
	return &Restager{
		f:       f,
		d:       d,
		log:     log,
		apiAddr: apiAddr,
		appID:   appID,
	}
}

func (r *Restager) SetAndRestage() {
	// Save refresh token in env variable
	setEnv := struct {
		Env struct {
			RefreshToken string `json:"REFRESH_TOKEN"`
		} `json:"environment_json"`
	}{
		Env: struct {
			RefreshToken string `json:"REFRESH_TOKEN"`
		}{
			RefreshToken: r.f.Token(),
		},
	}

	data, err := json.Marshal(setEnv)
	if err != nil {
		r.log.Panicf("failed to marshal: %s", err)
	}

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("%s/v2/apps/%s", r.apiAddr, r.appID),
		bytes.NewReader(data),
	)
	if err != nil {
		r.log.Fatalf("failed to create request: %s", err)
	}
	resp, err := r.d.Do(req)
	if err != nil {
		r.log.Fatalf("failed to set env variable: %s", err)
	}

	if resp.StatusCode != http.StatusCreated {
		r.log.Fatalf("failed to set env variable: %d", resp.StatusCode)
	}

	// Restage
	req, err = http.NewRequest(
		"POST",
		fmt.Sprintf("%s/v2/apps/%s/restage", r.apiAddr, r.appID),
		nil,
	)
	if err != nil {
		r.log.Fatalf("failed to create request: %s", err)
	}
	resp, err = r.d.Do(req)
	if err != nil {
		r.log.Fatalf("failed to restage: %s", err)
	}

	if resp.StatusCode != http.StatusCreated {
		r.log.Fatalf("failed to restage: %d", resp.StatusCode)
	}
}
