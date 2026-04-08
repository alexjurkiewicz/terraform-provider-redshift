# Design: Data API Support for Provisioned Redshift Clusters

**Date:** 2026-04-08
**Status:** Approved

## Overview

Extend the provider's `data_api` block to support provisioned Redshift clusters (identified by a `cluster_identifier`) in addition to the existing serverless workgroup support. The underlying SQL driver (`redshift-data-sql-driver`) already handles both DSN formats; this change wires up the provider schema and config-building logic.

## Schema Changes (`provider.go`)

The `data_api` block gains two new fields and `workgroup_name` becomes optional:

| Field | Type | Status | Notes |
|---|---|---|---|
| `workgroup_name` | string | Optional | `ExactlyOneOf: [workgroup_name, cluster_identifier]` |
| `cluster_identifier` | string | Optional (new) | `ExactlyOneOf: [workgroup_name, cluster_identifier]` |
| `username` | string | Optional (new) | Required at runtime when `cluster_identifier` is set |
| `region` | string | Required | Unchanged |

`ExactlyOneOf` ensures plan-time validation — exactly one of the two target fields must be set. The `username` field is optional in the schema but validated in `getConfigFromDataApiResourceData` to be non-empty when `cluster_identifier` is used (cross-field validation at config time).

The `data_api` block description is updated to remove the "serverless only" wording.

## Config Building (`config_data_api.go`)

Two DSN formats, both using the `redshift-data` driver:

- **Workgroup (existing):** `workgroup(name)/database?region=...&transactionMode=non-transactional&requestMode=blocking`
- **Cluster (new):** `username@cluster(clusterIdentifier)/database?region=...&transactionMode=non-transactional&requestMode=blocking`

### New functions

```go
func NewDataApiClusterConfig(clusterIdentifier, username, database, awsRegion string, maxConns int) *Config
func buildConnStrFromDataApiClusterConfig(clusterIdentifier, username, database, awsRegion string) string
```

### Updated function

`getConfigFromDataApiResourceData` gains a branch:
- `workgroup_name` set → existing workgroup path (no change)
- `cluster_identifier` set → validate `username` is non-empty, call `NewDataApiClusterConfig`
- Neither set → return error (should not happen due to `ExactlyOneOf`, but kept as a defensive check)

No changes to `config.go`, `provider.go`'s `providerConfigure`, or any resource files.

## Testing

- New file `config_data_api_test.go` (or added to existing test file if present):
  - Cluster DSN built correctly from `cluster_identifier` + `username` + `region`
  - Error returned when `cluster_identifier` set but `username` is empty
  - Existing workgroup path unchanged (regression)
- No acceptance tests (require live AWS infrastructure).

## Documentation

- Update `data_api` block description in `provider.go` to remove "serverless only" wording.
- Update `templates/index.md.tmpl` (if it exists) to add a provisioned cluster example alongside the serverless example.

## Out of Scope

- IAM authentication / assume-role for the Data API cluster path (can be a follow-up)
- Acceptance tests against a live provisioned cluster
