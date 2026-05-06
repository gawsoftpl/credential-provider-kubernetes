package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/kubelet/pkg/apis/credentialprovider/v1"
)

func TestHandle_Success(t *testing.T) {
	// 1. Setup Mock Server
	expectedSecret := "harbor-robot-token-123"
	expectedUser := "robot$test"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer mock-sa-token" {
			t.Errorf("Expected Bearer mock-sa-token, got %s", authHeader)
		}

		// Send Mock Response
		resp := TokenResponse{
			Username: expectedUser,
			Secret:   expectedSecret,
			Expires:  3600,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 2. Setup Environment
	t.Setenv("HARBOR_JWKS_BROKER_URL", server.URL)

	// 3. Prepare Input Request
	request := v1.CredentialProviderRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       "CredentialProviderRequest",
		},
		Image:               "harbor.example.com/project/image:latest",
		ServiceAccountToken: "mock-sa-token",
	}
	stdin := jsonReader(t, request)
	var stdout bytes.Buffer

	// 4. Run Handle
	err := handle("default-user", stdin, &stdout)
	if err != nil {
		t.Fatalf("handle failed: %v", err)
	}

	// 5. Verify Output
	var response v1.CredentialProviderResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	auth, ok := response.Auth["harbor.example.com"]
	if !ok {
		t.Fatal("registry key missing in response auth")
	}

	if auth.Username != expectedUser {
		t.Errorf("expected username %s, got %s", expectedUser, auth.Username)
	}
	if auth.Password != expectedSecret {
		t.Errorf("expected secret %s, got %s", expectedSecret, auth.Password)
	}
}

func TestHandle_RetryLogic(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(TokenResponse{Secret: "success-after-retry"})
	}))
	defer server.Close()

	t.Setenv("HARBOR_JWKS_BROKER_URL", server.URL)

	request := v1.CredentialProviderRequest{
		TypeMeta:            metav1.TypeMeta{APIVersion: v1.SchemeGroupVersion.String(), Kind: "CredentialProviderRequest"},
		Image:               "harbor.io/img",
		ServiceAccountToken: "token",
	}

	var stdout bytes.Buffer
	err := handle("jwt", jsonReader(t, request), &stdout)
	if err != nil {
		t.Fatalf("should have succeeded after retries: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestHandle_MissingToken(t *testing.T) {
	request := v1.CredentialProviderRequest{
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       "CredentialProviderRequest",
		},
		Image: "harbor.io/img",
		// ServiceAccountToken is empty
	}

	err := handle("jwt", jsonReader(t, request), io.Discard)
	if err == nil || !strings.Contains(err.Error(), "did not contain a service account token") {
		t.Errorf("expected error about missing token, got: %v", err)
	}
}

func TestRegistryHost(t *testing.T) {
	tests := []struct {
		image string
		want  string
	}{
		{"my.registry.com/repo/img:v1", "my.registry.com"},
		{"localhost:5000/img@sha256:123", "localhost:5000"},
		{"harbor.io/project/sub/img", "harbor.io"},
		{"nginx", "nginx"},
	}

	for _, tt := range tests {
		got := registryHost(tt.image)
		if got != tt.want {
			t.Errorf("registryHost(%q) = %q; want %q", tt.image, got, tt.want)
		}
	}
}

// Helper to convert struct to io.Reader
func jsonReader(t *testing.T, val any) io.Reader {
	t.Helper()
	data, err := json.Marshal(val)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(data)
}
