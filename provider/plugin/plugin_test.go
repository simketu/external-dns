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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/external-dns/endpoint"
)

func TestRecords(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{
			"dnsName" : "test.example.com"
		}]`))
	}))
	defer svr.Close()

	provider, err := NewPluginProvider(svr.URL)
	require.Nil(t, err)
	endpoints, err := provider.Records(context.TODO())
	require.Nil(t, err)
	require.NotNil(t, endpoints)
	require.Equal(t, []*endpoint.Endpoint{&endpoint.Endpoint{
		DNSName: "test.example.com",
	}}, endpoints)
}

func TestApplyChanges(t *testing.T) {
	successfulApplyChanges := true
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if successfulApplyChanges {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer svr.Close()

	provider, err := NewPluginProvider(svr.URL)
	require.Nil(t, err)
	err = provider.ApplyChanges(context.TODO(), nil)
	require.Nil(t, err)

	successfulApplyChanges = false

	err = provider.ApplyChanges(context.TODO(), nil)
	require.NotNil(t, err)
}
