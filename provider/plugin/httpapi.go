/*
Copyright 2023 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package plugin

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"

	log "github.com/sirupsen/logrus"
)

type HTTPProvider struct {
	provider provider.Provider
}

type PropertyValuesEqualsRequest struct {
	Name     string `json:"name"`
	Previous string `json:"previous"`
	Current  string `json:"current"`
}

type PropertyValuesEqualsResponse struct {
	Equals bool `json:"equals"`
}

func (p *HTTPProvider) recordsHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet { // records
		records, err := p.provider.Records(context.Background())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(records)
		return
	} else if req.Method == http.MethodPost { // applychanges
		var changes plan.Changes
		if err := json.NewDecoder(req.Body).Decode(&changes); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		err := p.provider.ApplyChanges(context.Background(), &changes)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	log.Errorf("Unsupported method %s", req.Method)
	w.WriteHeader(http.StatusBadRequest)
}

func (p *HTTPProvider) propertyValuesEqualHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		log.Errorf("Unsupported method %s", req.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	pve := PropertyValuesEqualsRequest{}
	if err := json.NewDecoder(req.Body).Decode(&pve); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	b := p.provider.PropertyValuesEqual(pve.Name, pve.Previous, pve.Current)
	r := PropertyValuesEqualsResponse{
		Equals: b,
	}
	out, err := json.Marshal(&r)
	if err != nil {
		log.Error(err)
	}
	w.Write(out)
}

func (p *HTTPProvider) adjustEndpointsHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		log.Errorf("Unsupported method %s", req.Method)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	pve := []*endpoint.Endpoint{}
	if err := json.NewDecoder(req.Body).Decode(&pve); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	pve = p.provider.AdjustEndpoints(pve)
	out, err := json.Marshal(&pve)
	if err != nil {
		log.Error(err)
	}
	w.Write(out)
}

func (p *HTTPProvider) negotiate(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(varyHeader, contentTypeHeader)
	w.Header().Set(contentTypeHeader, mediaTypeFormatAndVersion)
	w.WriteHeader(http.StatusOK)
}

// StartHTTPApi starts a HTTP server given any provider.
// the function takes an optional channel as input which is used to signal that the server has started.
// The server will listen on port 8888.
// The server will respond to the following endpoints:
// - /records (GET): returns the current records
// - /records (POST): applies the changes
// - /propertyvaluesequal (GET): executes the PropertyValuesEqual method
// - /adjustendpoints (GET): executes the AdjustEndpoints method
func StartHTTPApi(provider provider.Provider, startedChan chan struct{}) {
	p := HTTPProvider{
		provider: provider,
	}

	m := http.NewServeMux()
	m.HandleFunc("/", p.negotiate)
	m.HandleFunc("/records", p.recordsHandler)
	m.HandleFunc("/propertyvaluesequal", p.propertyValuesEqualHandler)
	m.HandleFunc("/adjustendpoints", p.adjustEndpointsHandler)

	// create a new http server
	s := &http.Server{
		Addr:    ":8888",
		Handler: m,
		// set timeouts so that a slow or malicious client doesn't
		// hold resources forever
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	l, err := net.Listen("tcp", ":8888")
	if err != nil {
		log.Fatal(err)
	}

	if startedChan != nil {
		startedChan <- struct{}{}
	}

	if err := s.Serve(l); err != nil {
		log.Fatal(err)
	}
}
