package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	log.Println("Starting sample...")
	defer log.Println("Closing sample...")
	api := getAPI()

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", os.Getenv("PORT")), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v3/apps", api), nil)
		if err != nil {
			log.Panicf("failed to create request: %s", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatalf("error while requesting from API: %s", err)
		}

		defer func() {
			io.Copy(ioutil.Discard, resp.Body)
			resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
			return
		}

		var response struct {
			Resources []struct {
				Name string `json:"name"`
			} `json:"resources"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf(`{"error":"failed to unmarshal response: %q"}`, err)))
			return
		}

		var result struct {
			Names []string `json:"names"`
		}

		for _, r := range response.Resources {
			result.Names = append(result.Names, r.Name)
		}

		data, err := json.Marshal(&result)
		if err != nil {
			log.Panicf("failed to marshal data: %s", err)
		}

		w.Write(data)
	})))
}

func getAPI() string {
	vcap := os.Getenv("VCAP_APPLICATION")
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(vcap), &m); err != nil {
		log.Fatalf("failed to unmarshal VCAP_APPLICATION: %s", err)
	}

	api, ok := m["cf_api"].(string)
	if !ok {
		log.Fatal("failed to unmarshal VCAP_APPLICATION: unable to find 'cf_api'")
	}

	return strings.Replace(api, "https", "http", 1)
}
