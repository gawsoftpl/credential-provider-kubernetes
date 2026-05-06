/*
Copyright 2025 The Kubernetes Authors.

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

package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/kubelet/pkg/apis/credentialprovider/v1"
)

var version = "dev"

//go:embed README.md
var readme string

func main() {
	flagset := flag.NewFlagSet("", flag.ExitOnError)
	flagset.Usage = func() { fmt.Fprintln(os.Stderr, strings.TrimSpace(readme)) }
	username := flagset.String("username", "jwt", "optionally set the username in the returned registry credentials")
	showVersion := flagset.Bool("version", false, "print version and exit")

	if err := flagset.Parse(os.Args[1:]); err != nil {
		exit(err)
	}

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if args := flagset.Args(); len(args) > 0 {
		exit(fmt.Errorf("unexpected args: %v", args))
	}

	if term.IsTerminal(int(os.Stdin.Fd())) {
		flagset.Usage()
		os.Exit(1)
	}

	if err := handle(*username, os.Stdin, os.Stdout); err != nil {
		exit(err)
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// registryHost extracts the registry hostname from an image reference.
// For example, "registry.example.com/library/nginx:latest" returns "registry.example.com".
func registryHost(image string) string {
	// Remove tag or digest
	if idx := strings.LastIndex(image, "@"); idx != -1 {
		image = image[:idx]
	}
	if idx := strings.LastIndex(image, ":"); idx != -1 {
		// Only strip if the colon is after the last slash (i.e., it's a tag, not a port)
		if slashIdx := strings.LastIndex(image, "/"); slashIdx < idx {
			image = image[:idx]
		}
	}

	// The first path component is the registry host
	if idx := strings.Index(image, "/"); idx != -1 {
		return image[:idx]
	}
	return image
}

func handle(username string, stdin io.Reader, stdout io.Writer) error {
	decoder := json.NewDecoder(stdin)

	request := &v1.CredentialProviderRequest{}
	err := decoder.Decode(&request)
	if err != nil {
		return fmt.Errorf("error parsing input: %w", err)
	}

	if request.APIVersion != v1.SchemeGroupVersion.String() || request.Kind != "CredentialProviderRequest" {
		return fmt.Errorf("only %v input is supported, got %v, Kind=%v", v1.SchemeGroupVersion.WithKind("CredentialProviderRequest"), request.APIVersion, request.Kind)
	}
	if request.ServiceAccountToken == "" {
		return fmt.Errorf("input did not contain a service account token")
	}

	registry := registryHost(request.Image)

	// Fetch credentials from the external broker
	tokenData, err := fetchCredentials(request.ServiceAccountToken)
	if err != nil {
		return fmt.Errorf("credential fetch failed: %w", err)
	}

	if tokenData.Secret == "" {
		return fmt.Errorf("credential fetch returned empty username or secret")
	}

	if tokenData.Username != "" {
		username = tokenData.Username
	}

	response := &v1.CredentialProviderResponse{
		TypeMeta:      metav1.TypeMeta{APIVersion: v1.SchemeGroupVersion.String(), Kind: "CredentialProviderResponse"},
		CacheKeyType:  v1.RegistryPluginCacheKeyType,
		CacheDuration: &metav1.Duration{},
		Auth:          map[string]v1.AuthConfig{registry: {Username: username, Password: tokenData.Secret}},
	}
	return json.NewEncoder(stdout).Encode(response)
}

type TokenResponse struct {
	Username   string `json:"username"`
	Secret     string `json:"secret"`
	Expires    int64  `json:"expires"`
	RevokeCode string `json:"revoke_code"`
	ID         int64  `json:"id"`
}

func fetchCredentials(token string) (*TokenResponse, error) {
	url := os.Getenv("HARBOR_JWKS_BROKER_URL")
	if url == "" {
		url = "http://localhost:8080"
	}

	// Get TTL from ENV
	ttlStr := os.Getenv("PASSWORD_DURATION_TTL")
	ttl, _ := strconv.Atoi(ttlStr)

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Prepare JSON payload for the duration
	payload := map[string]interface{}{
		"duration": ttl,
	}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for i := 0; i < 3; i++ {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return nil, err
		}

		// Set Headers
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			timeSleep := time.Duration(500 * (i + 1))
			time.Sleep(timeSleep * time.Millisecond)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			// Read body for better error context if available
			body, _ := io.ReadAll(resp.Body)
			lastErr = fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
			continue
		}

		var tokenResp TokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		return &tokenResp, nil
	}

	return nil, fmt.Errorf("failed to fetch credentials after 3 attempts: %v", lastErr)
}
