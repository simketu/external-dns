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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"

	log "github.com/sirupsen/logrus"
)

type PluginProvider struct {
	client          *http.Client
	remoteServerURL *url.URL
}

type PropertyValuesEqualsRequest struct {
	Name     string `json:"name"`
	Previous string `json:"previous"`
	Current  string `json:"current"`
}

type PropertiesValuesEqualsResponse struct {
	Equals bool `json:"equals"`
}

func NewPluginProvider(u string) (*PluginProvider, error) {
	parsedURL, err := url.Parse(u)
	if err != nil {
		return nil, err
	}
	return &PluginProvider{
		client:          &http.Client{},
		remoteServerURL: parsedURL,
	}, nil
}

// Records will make a GET call to remoteServerURL/records and return the results
func (p PluginProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	u, err := url.JoinPath(p.remoteServerURL.String(), "records")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	endpoints := []*endpoint.Endpoint{}
	err = json.Unmarshal(b, &endpoints)
	if err != nil {
		return nil, err
	}
	return endpoints, nil
}

// ApplyChanges will make a POST to remoteServerURL/records with the changes
func (p PluginProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	u, err := url.JoinPath(p.remoteServerURL.String(), "records")
	if err != nil {
		return err
	}
	b, err := json.Marshal(changes)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", u, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to apply changes with code %d", resp.StatusCode)
	}
	return nil
}

// PropertyValuesEqual will call the provider doing a GET on `/propertyvaluesequal` which will return a boolean in the format
// `{propertyvaluesequal: true}`
// Errors in anything technically happening from the provider will default to the default implmentation `previous == current`.
// Errors will also be logged and exposed as metrics so that it is possible to alert on the if needed.
//
// TODO(Raffo) this defaulting to the default behavior isn't ideal and could lead to misbehavior. I did this mostly because
// I have no better choice than doing this as we are "bending" the provider interface to work across the wire, exposing some
// of the limits of the provider interface itself. I think this is an opportunity for thinking if this requires a refactor
// as the quirks in its implementation seems to tell me that this is not the right interface to have to abstract a provider
// and rather a biproduct of the organic code of this project and its providers over the years.
func (p PluginProvider) PropertyValuesEqual(name string, previous string, current string) bool {
	u, err := url.JoinPath(p.remoteServerURL.String(), "propertiesvaluesequal")
	if err != nil {
		return previous == current
	}
	b, err := json.Marshal(&PropertyValuesEqualsRequest{
		Name:     name,
		Previous: previous,
		Current:  current,
	})
	if err != nil {
		return previous == current
	}

	req, err := http.NewRequest("GET", u, bytes.NewBuffer(b))
	if err != nil {
		return previous == current
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return previous == current
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorf("failed to apply changes with code %d", resp.StatusCode)
		return previous == current
	}

	respoBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("failed to apply changes with code %d", resp.StatusCode)
		return previous == current
	}

	r := PropertiesValuesEqualsResponse{}
	err = json.Unmarshal(respoBody, &r)
	if err != nil {
		log.Errorf("failed to apply changes with code %d", resp.StatusCode)
		return previous == current
	}
	return r.Equals
}

// AdjustEndpoints will call the provider doing a GET on `/adjustendpoints` which will return a list of modified endpoints
// based on a provider specific requirement.
// This method returns the original list of endpoints e, non adjusted if there is a technical error on the provider's side.
// This is again one evidence of how this interface was not made to be used across the wire and we have to assume a default case
// of errors that may not be safe.
// TODO revisit the decision around error handling in this method and the interface in general.
func (p PluginProvider) AdjustEndpoints(e []*endpoint.Endpoint) []*endpoint.Endpoint {
	u, err := url.JoinPath(p.remoteServerURL.String(), "adjustendpoints")
	if err != nil {
		return e
	}
	b, err := json.Marshal(e)
	if err != nil {
		return e
	}
	req, err := http.NewRequest("POST", u, bytes.NewBuffer(b))
	if err != nil {
		return e
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return e
	}
	defer resp.Body.Close()

	b, err = io.ReadAll(resp.Body)
	if err != nil {
		return e
	}

	endpoints := []*endpoint.Endpoint{}
	err = json.Unmarshal(b, &endpoints)
	if err != nil {
		return e
	}
	return endpoints
}

// GetDomainFilter is the default implementation of GetDomainFilter.
func (p PluginProvider) GetDomainFilter() endpoint.DomainFilterInterface {
	return endpoint.DomainFilter{}
}
