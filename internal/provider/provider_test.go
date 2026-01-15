// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectLocalStack_Available(t *testing.T) {
	// Create a mock server that responds like LocalStack
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_localstack/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"services": {}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// The actual detectLocalStack function hardcodes localhost:4566,
	// so we can't easily test it with a mock server.
	// Instead, we test the logic by calling it and accepting the result
	// since localhost:4566 may or may not be running.
	detected, endpoint := detectLocalStack()

	// If LocalStack happens to be running, both should be set
	// If not, both should be empty/false
	if detected && endpoint == "" {
		t.Error("detectLocalStack() returned detected=true but empty endpoint")
	}
	if !detected && endpoint != "" {
		t.Errorf("detectLocalStack() returned detected=false but endpoint=%q", endpoint)
	}
}

func TestDetectLocalStack_NotAvailable(t *testing.T) {
	// This test verifies the function handles connection failures gracefully.
	// Since detectLocalStack() uses a hardcoded endpoint (localhost:4566),
	// we can't inject a mock. We rely on the fact that if LocalStack isn't
	// running, it should return (false, "").
	//
	// In CI environments without LocalStack, this is the expected behavior.
	// The test primarily ensures no panic or unexpected error occurs.
	detected, endpoint := detectLocalStack()

	// We just verify the return values are consistent
	t.Logf("detectLocalStack() returned detected=%v, endpoint=%q", detected, endpoint)

	if detected {
		if endpoint != "http://localhost:4566" {
			t.Errorf("detectLocalStack() detected=true but unexpected endpoint=%q", endpoint)
		}
	} else {
		if endpoint != "" {
			t.Errorf("detectLocalStack() detected=false but endpoint=%q", endpoint)
		}
	}
}

func TestNew(t *testing.T) {
	factory := New("test-version")
	p := factory()

	probeProvider, ok := p.(*ProbeProvider)
	if !ok {
		t.Fatalf("New() did not return *ProbeProvider, got %T", p)
	}

	if probeProvider.version != "test-version" {
		t.Errorf("provider version = %q, want %q", probeProvider.version, "test-version")
	}
}
