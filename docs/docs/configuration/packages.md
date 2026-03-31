---
sidebar_position: 3
---

# Packages

Stroppy Cloud uses a flexible package management system that supports both Debian (apt) and Red Hat (rpm) package managers, custom repositories, and raw package file URLs.

Defined in `internal/domain/types/packages.go`.

## PackageSet

A `PackageSet` defines everything needed to install a component on a target machine:

```go
type PackageSet struct {
    Apt           []string `json:"apt,omitempty"`
    Rpm           []string `json:"rpm,omitempty"`
    PreInstallApt []string `json:"pre_install_apt,omitempty"`
    PreInstallRpm []string `json:"pre_install_rpm,omitempty"`
    CustomRepoApt string   `json:"custom_repo_apt,omitempty"`
    CustomRepoKey string   `json:"custom_repo_key,omitempty"`
    CustomRepoRpm string   `json:"custom_repo_rpm,omitempty"`
    DebFiles      []string `json:"deb_files,omitempty"`
    RpmFiles      []string `json:"rpm_files,omitempty"`
}
```

| Field | Description |
|-------|-------------|
| `apt` | Debian/Ubuntu package names to install via `apt-get install` |
| `rpm` | RHEL/CentOS package names to install via `dnf install` |
| `pre_install_apt` | Shell commands to run before apt install (add repos, import keys, update cache) |
| `pre_install_rpm` | Shell commands to run before rpm install |
| `custom_repo_apt` | Full apt repository line (e.g., `deb https://my-registry.corp/apt jammy main`) |
| `custom_repo_key` | URL to the GPG key for the custom apt repo |
| `custom_repo_rpm` | Full yum/dnf repo baseurl for custom rpm repo |
| `deb_files` | URLs to raw `.deb` files to download and install with `dpkg -i` |
| `rpm_files` | URLs to raw `.rpm` files to download and install with `rpm -i` |

## PackageDefaults

The server maintains default packages for all components, keyed by database version:

```go
type PackageDefaults struct {
    Postgres   map[string]PackageSet `json:"postgres"`
    MySQL      map[string]PackageSet `json:"mysql"`
    Picodata   map[string]PackageSet `json:"picodata"`
    Monitoring PackageSet            `json:"monitoring"`
    Stroppy    PackageSet            `json:"stroppy"`
}
```

## Built-in Defaults

### PostgreSQL 16

```json
{
  "apt": ["postgresql-16", "postgresql-client-16"],
  "pre_install_apt": [
    "sh -c 'echo \"deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main\" > /etc/apt/sources.list.d/pgdg.list'",
    "wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -",
    "apt-get update"
  ],
  "rpm": ["postgresql16-server", "postgresql16"],
  "pre_install_rpm": [
    "dnf install -y https://download.postgresql.org/pub/repos/yum/reporpms/EL-$(rpm -E %rhel)-x86_64/pgdg-redhat-repo-latest.noarch.rpm"
  ]
}
```

### PostgreSQL 17

Same structure as 16 with package names updated to `postgresql-17` / `postgresql17-server`.

### MySQL 8.0 / 8.4

```json
{
  "apt": ["mysql-server-8.0", "mysql-client"],
  "rpm": ["mysql-community-server", "mysql-community-client"]
}
```

### Picodata 25.3

```json
{
  "apt": ["picodata"],
  "pre_install_apt": [
    "curl -fsSL https://download.picodata.io/tarantool-picodata/picodata.gpg.key | gpg --no-default-keyring --keyring gnupg-ring:/etc/apt/trusted.gpg.d/picodata.gpg --import && chmod 644 /etc/apt/trusted.gpg.d/picodata.gpg",
    "echo \"deb https://download.picodata.io/tarantool-picodata/ubuntu/ $(lsb_release -cs) main\" > /etc/apt/sources.list.d/picodata.list",
    "apt-get update"
  ],
  "rpm": ["picodata"],
  "pre_install_rpm": ["(add picodata yum repo)"]
}
```

### Monitoring and Stroppy

Monitoring agents (node_exporter, vmagent, otel-collector) and stroppy are installed from binary tarballs/GitHub releases, not system packages. Their `PackageSet` entries are empty by default.

## Override Hierarchy

Package resolution follows this order:

1. **Per-run override** -- If `RunConfig.Packages` is non-nil, it is used for the current run.
2. **Admin defaults** -- Managed via `PUT /api/v1/admin/packages`. Persisted in server settings.
3. **Built-in defaults** -- Hardcoded in `DefaultPackages()`.

## Using Custom Packages

### Via custom repository

```bash
curl -X PUT http://localhost:8080/api/v1/admin/packages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer my-secret-key" \
  -d '{
    "postgres": {
      "16": {
        "apt": ["my-postgres-16"],
        "custom_repo_apt": "deb https://registry.internal/apt jammy main",
        "custom_repo_key": "https://registry.internal/key.gpg"
      }
    },
    "mysql": {},
    "picodata": {},
    "monitoring": {},
    "stroppy": {}
  }'
```

### Via uploaded .deb files

First upload the package:

```bash
curl -X POST http://localhost:8080/api/v1/upload/deb \
  -H "Authorization: Bearer my-secret-key" \
  -F "file=@my-custom-postgres_16.1-1_amd64.deb"
```

Then reference it in the packages config:

```json
{
  "postgres": {
    "16": {
      "deb_files": ["http://server:8080/packages/my-custom-postgres_16.1-1_amd64.deb"]
    }
  }
}
```

### Per-run override

Include the `packages` field directly in the `RunConfig`:

```json
{
  "id": "custom-pkg-run",
  "provider": "docker",
  "packages": {
    "apt": ["my-custom-postgres"],
    "deb_files": ["http://server:8080/packages/custom.deb"]
  },
  "database": {
    "kind": "postgres",
    "version": "16",
    "postgres": {
      "master": {"role": "database", "count": 1, "cpus": 2, "memory_mb": 4096, "disk_gb": 50}
    }
  }
}
```
