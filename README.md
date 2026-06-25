# ExternalDNS - Yandex Cloud DNS Webhook

This is an [ExternalDNS provider](https://github.com/kubernetes-sigs/external-dns/blob/master/docs/tutorials/webhook-provider.md) for [Yandex Cloud DNS](https://cloud.yandex.com/en/services/dns).
This projects externalizes the provider for Yandex Cloud DNS and offers a way forward for bugfixes.

## Installation

This webhook provider is run easiest as sidecar within the `external-dns` pod. This can be achieved using the official
`external-dns` Helm chart and [its support for the `webhook` provider type]([https://kubernetes-sigs.github.io/external-dns/latest/charts/external-dns/#providers]).

Setting the `provider.name` to `webhook` allows configuration of the
`external-dns-yandex-webhook` via a few additional values:

```yaml
provider:
  name: webhook
  webhook:
    image:
      repository: ghcr.io/drengskapr/external-dns-yandex-webhook
      tag: 1.0.0
    args:
      - --folder-id=YOUR_FOLDER_ID
      - --auth-key-file=/etc/kubernetes/key.json
    extraVolumeMounts:
      - name: yandexconfig
        mountPath: /etc/kubernetes/
    resources: {}
    securityContext:
      runAsUser: 1000
```

The referenced `extraVolumeMount` points to a `Secret` containing the service account key file for Yandex Cloud authentication.

## Command Line Arguments

The webhook requires the following command line arguments:

- `--folder-id`: Yandex Cloud folder ID where your DNS zones are located.
- `--auth-key-file`: Path to the Yandex Cloud service account key file.

## Authentication

For authentication, this webhook uses a service account key file. To create one:

1. Create a service account in Yandex Cloud with the necessary permissions for DNS management
2. Create a service account key using the Yandex Cloud CLI:

```shell
# Install Yandex Cloud CLI if you haven't already
# https://cloud.yandex.com/en/docs/cli/quickstart

# Create the IAM key JSON file
yc iam key create iamkey \
  --service-account-id=<your service account ID> \
  --format=json \
  --output=key.json
```

3. Add this file to your Kubernetes Secret

Create a Secret with the service account key file:

```shell
kubectl create secret generic yandexconfig --namespace external-dns --from-file=key.json
```

and then add it as an extraVolume to within the `values.yaml` of external-dns:

```yaml
extraVolumes:
  - name: yandexconfig
    secret:
      secretName: yandexconfig
```

## Build

Build the binary locally (output: `build/bin/external-dns-yandex-webhook`):

```shell
make build
```

Or compile all packages directly:

```shell
go build ./...
```

Build a container image from source using the provided multi-stage `Dockerfile`:

```shell
docker build -t external-dns-yandex-webhook:local .
```

Release images are published to `ghcr.io/<owner>/external-dns-yandex-webhook`. Pushing a `v*` git tag triggers the GitHub Actions `release` job, which builds the `linux/amd64` image from the multi-stage `Dockerfile` and pushes it tagged with the version — the tag minus its leading `v` (e.g. `v1.2.3` → `:1.2.3`). The image namespace is derived from the repository owner (`GITHUB_REPOSITORY_OWNER`), which GitHub Actions sets automatically.

## Testing

Run the full test suite:

```shell
go test ./...
```

Run a single test (add `-count=1` to bypass the test cache):

```shell
go test -v -run TestYandexProvider_Records ./internal/yandex/provider
```

Linting matches CI via `golangci-lint`:

```shell
golangci-lint run --exclude-files ".*_test.go"
```
