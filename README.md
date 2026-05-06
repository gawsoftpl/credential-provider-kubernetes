# credential-provider-kubernetes

`credential-provider-kubernetes` is an image credential provider plugin for Kubernetes that exchanges Kubernetes service account tokens for temporary Harbor registry credentials via an external credential broker.

## Usage

```bash
request='{"apiVersion":"credentialprovider.kubelet.k8s.io/v1","kind":"CredentialProviderRequest","image":"...","serviceAccountToken":"..."}'
HARBOR_JWKS_BROKER_URL="http://localhost:8080" PASSWORD_DURATION_TTL="3600" echo "${request}" | credential-provider-kubernetes [--username=USER]
```

`credential-provider-kubernetes` is called with `STDIN` of a JSON-serialized `credentialprovider.kubelet.k8s.io/v1` `CredentialProviderRequest`, which must contain a `serviceAccountToken` value. For example:

```json
{
  "apiVersion": "credentialprovider.kubelet.k8s.io/v1",
  "kind": "CredentialProviderRequest",
  "image": "...",
  "serviceAccountToken": "..."
}
```

## Configuration

To configure this credential plugin on a node:

**1. Install the binary**
Create a directory for credential plugin binaries, and place this binary in that directory.

**2. Create a configuration file**
Create a `CredentialProviderConfig` configuration file containing:

```yaml
kind: CredentialProviderConfig
apiVersion: kubelet.config.k8s.io/v1
providers:
- name: credential-provider-kubernetes
  apiVersion: credentialprovider.kubelet.k8s.io/v1
  tokenAttributes:
    requireServiceAccount: true
    serviceAccountTokenAudience: "" # TODO: replace with your Harbor registry hostname
  
  matchImages:
  - "" # TODO: replace with your Harbor registry hostname(s)

  defaultCacheDuration: "1h"

  # Optionally specify the default fallback username, default is "jwt"
  args:
  - "--username=jwt"
  
  # Configure the connection to your broker
  env:
  - name: HARBOR_JWKS_BROKER_URL
    value: "http://localhost:8080" # TODO: replace with your broker URL
  - name: PASSWORD_DURATION_TTL
    value: "3600"
```

*Note: `defaultCacheDuration` is required by the kubelet config API as a fallback. However, this plugin dynamically dictates the cache duration to the kubelet based on the `expires` value returned by the credential broker to ensure tokens are not cached beyond their valid lifetime.*

**3. Adjust Kubelet flags**
Adjust the kubelet startup flags to point at your binary directory and configuration file:

```text
--image-credential-provider-bin-dir=/path/to/step-1/directory
--image-credential-provider-config=/path/to/step-2/file
```
