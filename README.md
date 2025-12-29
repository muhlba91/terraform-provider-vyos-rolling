# VyOS Provider for Terraform / OpenTofu

<!-- ![Build Status](https://github.com/echowings/terraform-provider-vyos-rolling/actions/workflows/XYZ.yml/badge.svg) -->
[![Go Report Card](https://goreportcard.com/badge/github.com/echowings/terraform-provider-vyos)](https://goreportcard.com/report/github.com/echowings/terraform-provider-vyos)
<!-- ![GitHub release](https://img.shields.io/github/v/release/echowings/terraform-provider-vyos-rolling) -->

Configure VyOS rolling-release appliances through Terraform or OpenTofu using the
official HTTPS API. The provider is auto-generated from the upstream VyOS API
schemas so it always ships the full surface area exposed by the OS.

## Highlights

- Tracks the public rolling builds by pulling their schema definitions (see
	`data/vyos-1x-info.txt`).
- Generates provider docs under `docs/` and example configs under `examples/`
	straight from the API definitions.
- Supports every VyOS API resource, including wireguard, PPPoE, firewall,
	policy routing, DHCPv6 PD, and more.
- Compatible with Terraform `>= 1.0` and OpenTofu `>= 1.6` (tested with both).

## Getting Started

```hcl
terraform {
	required_version = ">= 1.0"
	required_providers {
		vyos = {
			source  = "echowings/vyos-rolling"
			version = "0.1.202507153"
		}
	}
}

variable "vyos_api_host" {
	type    = string
	default = "https://vyos.example"
}

variable "vyos_api_key" {
	type      = string
	sensitive = true
}

provider "vyos" {
	endpoint = var.vyos_api_host
	api_key  = var.vyos_api_key

	certificate = {
		disable_verify = true # optional: keep TLS even with self-signed certs
	}

	default_timeouts = 20 # minutes, overrides the 15m built-in default
}
```

Once the provider is configured you can reference any generated resource, e.g.:

```hcl
resource "vyos_system" "global" {
	host_name = "edge-1"
}

resource "vyos_interfaces_ethernet" "wan" {
	identifier = { ethernet = "eth0" }
	address    = ["dhcp"]
}
```

Detailed API docs for each resource and data source are in `docs/` (same format
that the Terraform Registry renders).

## Authentication and TLS

- **API key** — generated under `set service https api keys`. Supply it as the
	`api_key` argument or stitch secrets from Vault like the `vyos-iac` repo does.
- **Endpoint** — full `https://host` URL. The provider will call `/rest/config`.
- **TLS** — for lab routers, set `certificate.disable_verify = true`. In
	production, point the VyOS API to a CA-signed certificate instead.
- **Timeouts** — `default_timeouts` sets the per-operation timeout (minutes).
	Resource-specific overrides exist on certain long-running resources.

## Documentation and Examples

- `docs/index.md` describes the provider schema. Each resource has a Markdown
	page under `docs/resources/` and every data source under `docs/data-sources/`.
- Sample configs live in `examples/` and mirror the generated docs.
- The `vyos-iac` repo inside this workspace shows a full end-to-end usage
	pattern with YAML-driven modules.

### Operational actions (imperative)

Most resources in this provider are declarative configuration. A small set of
"operational" resources intentionally perform actions.

- `vyos_system_image_upgrade` installs a pinned ISO via the HTTPS API `/image`
	endpoint and can optionally reboot the device. Run it in a separate apply
	(for example `tofu apply -target=vyos_system_image_upgrade.<name>`) because a
	reboot will interrupt other operations.

## Development

Prerequisites:

- Go `1.24`+
- GNU Make, curl, jq, xmllint, Docker (for generating interface definitions)
- Java (for `tools/trang-20091111/trang.jar`)

Common tasks:

```bash
make build          # compile the provider for your platform
make test           # run unit tests
make docs           # regenerate Markdown docs in docs/
make install        # copy the provider into ~/.terraform.d/plugins
```

### Updating to a newer VyOS rolling build

1. `make data/vyos-1x-info.txt` — records the timestamp of the latest
	 successful nightly build.
2. `make internal/vyos/vyosinterfaces/autogen.go` — clones the VyOS repo,
	 generates XSD files, then produces Go structs for every CLI node.
3. `make generate` (see the Makefile) — refreshes provider schema JSON,
	 Terraform docs, and manifests.

After regenerating artifacts, run `make test` and update `CHANGELOG.md` before
submitting a PR. See [CONTRIBUTE.md](CONTRIBUTE.md) for the coding style and
release process details.
