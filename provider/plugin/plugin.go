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
	"sigs.k8s.io/external-dns/provider"
)

type PluginProvider struct {
	provider.BaseProvider
	client          *http.Client
	remoteServerURL *url.URL
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

// Records will make a GET call to p.remoteServerURL and return the results
func (p PluginProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	req, err := http.NewRequest("GET", p.remoteServerURL.String(), nil)
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

	fmt.Println(string(b))

	endpoints := []*endpoint.Endpoint{}
	err = json.Unmarshal(b, &endpoints)
	if err != nil {
		return nil, err
	}
	return endpoints, nil
}

// ApplyChanges will make a POST to p.remoteServerURL with the changes
func (p PluginProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	b, err := json.Marshal(changes)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", p.remoteServerURL.String(), bytes.NewBuffer(b))
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
