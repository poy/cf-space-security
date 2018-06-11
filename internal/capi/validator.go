package capi

import (
	"fmt"
	"log"
	"net/http"
)

type Validator struct {
	appID    string
	capiAddr string
	doer     Doer
	log      *log.Logger
}

func NewValidator(appID, capiAddr string, doer Doer, log *log.Logger) *Validator {
	return &Validator{
		appID:    appID,
		capiAddr: capiAddr,
		doer:     doer,
		log:      log,
	}
}

func (v *Validator) Validate(token string) bool {
	if token == "" {
		return false
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/v3/apps/%s", v.capiAddr, v.appID), nil)
	if err != nil {
		v.log.Fatalf("failed to create CAPI request: %s", err)
	}
	req.Header.Set("Authorization", token)

	resp, err := v.doer.Do(req)
	if err != nil {
		v.log.Printf("failed making request to CAPI (%s): %s", v.capiAddr, err)
		return false
	}

	return resp.StatusCode == http.StatusOK
}
