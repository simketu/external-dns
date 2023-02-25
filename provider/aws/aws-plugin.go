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

package aws

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

const (
	mediaTypeFormatAndVersion = "application/external.dns.plugin+json;version=1"
	contentTypeHeader         = "Content-Type"
	acceptHeader              = "Accept"
	varyHeader                = "Vary"
)

type AWSPlugin struct {
	provider *AWSProvider
}

type PropertyValuesEqualsRequest struct {
	Name     string `json:"name"`
	Previous string `json:"previous"`
	Current  string `json:"current"`
}

type PropertyValuesEqualsResponse struct {
	Equals bool `json:"equals"`
}

func (p *AWSPlugin) awsProviderHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet { // records
		log.Println("get records")
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
		log.Println("post applychanges")
		// extract changes from the request body
		var changes plan.Changes
		if err := json.NewDecoder(req.Body).Decode(&changes); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		p.provider.ApplyChanges(context.Background(), &changes)

		err := p.provider.ApplyChanges(context.Background(), &changes)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}
	log.Println("this should never happen")
}

func (p *AWSPlugin) propertyValuesEquals(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet { // propertyValuesEquals
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
			panic(err)
		}
		w.Write(out)
	}

}

func (p *AWSPlugin) adjustEndpoints(w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet { // propertyValuesEquals
		pve := []*endpoint.Endpoint{}
		if err := json.NewDecoder(req.Body).Decode(&pve); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		pve = p.provider.AdjustEndpoints(pve)
		out, _ := json.Marshal(&pve)
		w.Write(out)
	}

}

func (p *AWSPlugin) Negotiate(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(varyHeader, contentTypeHeader)
	w.Header().Set(contentTypeHeader, mediaTypeFormatAndVersion)
	w.WriteHeader(200)
}

func StartAWSPluginProvider(config AWSConfig, startedChan chan struct{}) {
	// instantiate the aws provider
	awsProvider, err := NewAWSProvider(config)
	if err != nil {
		panic(err)
	}

	p := AWSPlugin{
		provider: awsProvider,
	}

	m := http.NewServeMux()
	m.HandleFunc("/", p.Negotiate)
	m.HandleFunc("/records", p.awsProviderHandler)
	m.HandleFunc("/propertyvaluesequal", p.propertyValuesEquals)
	m.HandleFunc("/adjustendpoints", p.adjustEndpoints)

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

	startedChan <- struct{}{}

	if err := s.Serve(l); err != nil {
		log.Fatal(err)
	}
}
