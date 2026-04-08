# Data API Provisioned Cluster Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the `data_api` provider block to support provisioned Redshift clusters (via `cluster_identifier` + `username`) in addition to the existing serverless workgroup path.

**Architecture:** The underlying SQL driver (`redshift-data-sql-driver`) already supports both workgroup and cluster DSN formats. We add `cluster_identifier` and `username` fields to the `data_api` schema block (making `workgroup_name` optional with `ExactlyOneOf`), add a new cluster DSN builder in `config_data_api.go`, and update `getConfigFromDataApiResourceData` to branch on which field is set.

**Tech Stack:** Go, `github.com/hashicorp/terraform-plugin-sdk/v2`, `github.com/mmichaelb/redshift-data-sql-driver` (DSN format: `username@cluster(id)/db?region=...`)

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `redshift/config_data_api.go` | Modify | Add cluster DSN builder + update config resolver |
| `redshift/config_data_api_test.go` | Create | Unit tests for DSN building and config resolution |
| `redshift/provider.go` | Modify | Add `cluster_identifier` + `username` to `data_api` schema; make `workgroup_name` optional |
| `examples/provider/provider_using_redshift_data_api_provisioned.tf` | Create | Provisioned cluster usage example |
| `templates/index.md.tmpl` | Modify | Reference new example in docs |

---

## Task 1: Write failing tests for cluster DSN building and config resolution

**Files:**
- Create: `redshift/config_data_api_test.go`

- [ ] **Step 1: Create the test file with three failing tests**

```go
package redshift

import (
	"testing"
)

func TestBuildConnStrFromDataApiClusterConfig(t *testing.T) {
	got := buildConnStrFromDataApiClusterConfig("my-cluster", "myuser", "mydb", "us-east-1")
	want := "myuser@cluster(my-cluster)/mydb?region=us-east-1&transactionMode=non-transactional&requestMode=blocking"
	if got != want {
		t.Errorf("buildConnStrFromDataApiClusterConfig() = %q, want %q", got, want)
	}
}

func TestGetConfigFromDataApiResourceData_ClusterMissingUsername(t *testing.T) {
	// getConfigFromDataApiResourceData should return an error when cluster_identifier
	// is set but username is empty.
	_, err := newDataApiClusterConfig("my-cluster", "", "mydb", "us-east-1", 1)
	if err == nil {
		t.Fatal("expected error when username is empty, got nil")
	}
}

func TestBuildConnStrFromDataApiWorkgroupConfig_Unchanged(t *testing.T) {
	// Regression: existing workgroup DSN must not change.
	got := buildConnStrFromDataApiConfig("my-workgroup", "mydb", "ap-southeast-2")
	want := "workgroup(my-workgroup)/mydb?region=ap-southeast-2&transactionMode=non-transactional&requestMode=blocking"
	if got != want {
		t.Errorf("buildConnStrFromDataApiConfig() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run the tests — verify they fail**

```bash
cd /path/to/repo && go test ./redshift -run "TestBuildConnStrFromDataApiClusterConfig|TestGetConfigFromDataApiResourceData_ClusterMissingUsername|TestBuildConnStrFromDataApiWorkgroupConfig_Unchanged" -v
```

Expected: compilation error (`buildConnStrFromDataApiClusterConfig` and `newDataApiClusterConfig` undefined) or FAIL.

---

## Task 2: Implement cluster DSN builder and config constructor in `config_data_api.go`

**Files:**
- Modify: `redshift/config_data_api.go`

- [ ] **Step 1: Add `newDataApiClusterConfig` and `buildConnStrFromDataApiClusterConfig`**

Add the following to `redshift/config_data_api.go` after the existing `buildConnStrFromDataApiConfig` function:

```go
func newDataApiClusterConfig(clusterIdentifier, username, database, awsRegion string, maxConns int) (*Config, error) {
	if username == "" {
		return nil, fmt.Errorf("data_api configuration with cluster_identifier requires username to be set")
	}
	connStr := buildConnStrFromDataApiClusterConfig(clusterIdentifier, username, database, awsRegion)
	return NewConfig(redshiftDataDriverName, connStr, database, maxConns), nil
}

func buildConnStrFromDataApiClusterConfig(clusterIdentifier, username, database, awsRegion string) string {
	return fmt.Sprintf(
		"%s@cluster(%s)/%s?region=%s&transactionMode=non-transactional&requestMode=blocking",
		username, clusterIdentifier, database, awsRegion,
	)
}
```

- [ ] **Step 2: Update `getConfigFromDataApiResourceData` to handle both modes**

Replace the existing `getConfigFromDataApiResourceData` function body:

```go
func getConfigFromDataApiResourceData(d *schema.ResourceData, database string) (*Config, error) {
	workgroupName, workgroupNameOk := d.GetOk("data_api.0.workgroup_name")
	clusterIdentifier, clusterIdentifierOk := d.GetOk("data_api.0.cluster_identifier")
	region, regionOk := d.GetOk("data_api.0.region")

	if !regionOk {
		return nil, fmt.Errorf("data_api configuration requires region to be set")
	}

	if clusterIdentifierOk {
		username, _ := d.GetOk("data_api.0.username")
		return newDataApiClusterConfig(clusterIdentifier.(string), username.(string), database, region.(string), 1)
	}

	if workgroupNameOk {
		return NewDataApiConfig(workgroupName.(string), database, region.(string), 1), nil
	}

	return nil, fmt.Errorf("data_api configuration requires either workgroup_name or cluster_identifier to be set")
}
```

- [ ] **Step 3: Run the tests — verify they pass**

```bash
go test ./redshift -run "TestBuildConnStrFromDataApiClusterConfig|TestGetConfigFromDataApiResourceData_ClusterMissingUsername|TestBuildConnStrFromDataApiWorkgroupConfig_Unchanged" -v
```

Expected output:
```
--- PASS: TestBuildConnStrFromDataApiClusterConfig (0.00s)
--- PASS: TestGetConfigFromDataApiResourceData_ClusterMissingUsername (0.00s)
--- PASS: TestBuildConnStrFromDataApiWorkgroupConfig_Unchanged (0.00s)
PASS
```

- [ ] **Step 4: Run the full unit test suite**

```bash
make test
```

Expected: all tests pass (no regressions).

- [ ] **Step 5: Commit**

```bash
git add redshift/config_data_api.go redshift/config_data_api_test.go
git commit -m "feat: add Data API support for provisioned Redshift clusters"
```

---

## Task 3: Update the provider schema in `provider.go`

**Files:**
- Modify: `redshift/provider.go`

- [ ] **Step 1: Update the `data_api` block schema**

In `provider.go`, find the `data_api` block's `Elem` schema (around line 86). Make these changes:

1. Change `data_api` description from:
   ```go
   Description: "Configuration for using the Redshift Data API. This can only be used for serverless Redshift clusters.",
   ```
   to:
   ```go
   Description: "Configuration for using the Redshift Data API. Supports both serverless workgroups and provisioned clusters.",
   ```

2. Change `workgroup_name` from `Required: true` to `Optional: true` and add `ExactlyOneOf`:
   ```go
   "workgroup_name": {
       Type:        schema.TypeString,
       Optional:    true,
       Description: "The name of the Redshift Serverless workgroup to connect to.",
       DefaultFunc: schema.EnvDefaultFunc("REDSHIFT_DATA_API_SERVERLESS_WORKGROUP_NAME", nil),
       ValidateFunc: validation.All(
           validation.StringLenBetween(3, 64),
           validation.StringMatch(regexp.MustCompile("[a-z0-9-]+"), "must be lowercase alphanumeric or hyphen characters"),
       ),
       ExactlyOneOf: []string{"data_api.0.workgroup_name", "data_api.0.cluster_identifier"},
   },
   ```

3. Add `cluster_identifier` field after `workgroup_name`:
   ```go
   "cluster_identifier": {
       Type:        schema.TypeString,
       Optional:    true,
       Description: "The identifier of the provisioned Redshift cluster to connect to.",
       DefaultFunc: schema.EnvDefaultFunc("REDSHIFT_DATA_API_CLUSTER_IDENTIFIER", nil),
       ValidateFunc: validation.StringLenBetween(1, 63),
       ExactlyOneOf: []string{"data_api.0.workgroup_name", "data_api.0.cluster_identifier"},
   },
   ```

4. Add `username` field after `cluster_identifier`:
   ```go
   "username": {
       Type:        schema.TypeString,
       Optional:    true,
       Description: "The database user to connect as. Required when using cluster_identifier.",
       DefaultFunc: schema.EnvDefaultFunc("REDSHIFT_DATA_API_USERNAME", nil),
   },
   ```

5. Update `region` to be `Optional` (it already is) and update its description:
   ```go
   "region": {
       Type:        schema.TypeString,
       Required:    true,
       Description: "The AWS region where the Redshift workgroup or cluster is located.",
       DefaultFunc: schema.MultiEnvDefaultFunc([]string{"AWS_REGION", "AWS_DEFAULT_REGION"}, nil),
   },
   ```

- [ ] **Step 2: Update `getConfigFromResourceData` to detect cluster mode**

In `provider.go`, the `getConfigFromResourceData` function currently checks `d.GetOk("data_api.0.workgroup_name")` to decide if Data API is active. Update it to also trigger on `cluster_identifier`:

```go
func getConfigFromResourceData(d *schema.ResourceData, temporaryCredentialsResolver temporaryCredentialsResolverFunc) (*Config, error) {
	database := d.Get("database").(string)
	maxConnections := d.Get("max_connections").(int)
	_, useDataApiWorkgroup := d.GetOk("data_api.0.workgroup_name")
	_, useDataApiCluster := d.GetOk("data_api.0.cluster_identifier")
	useDataApi := useDataApiWorkgroup || useDataApiCluster
	_, usePqResourceData := d.GetOk("host")

	if useDataApi && usePqResourceData {
		return nil, fmt.Errorf("using both auth methods 'data_api' and 'host' is not allowed")
	}
	if useDataApi {
		return getConfigFromDataApiResourceData(d, database)
	}
	return getConfigFromPqResourceData(d, database, maxConnections, temporaryCredentialsResolver)
}
```

- [ ] **Step 3: Verify provider internal validation passes**

```bash
go test ./redshift -run TestProvider -v
```

Expected:
```
--- PASS: TestProvider (0.00s)
--- PASS: TestProvider_impl (0.00s)
PASS
```

- [ ] **Step 4: Run full unit test suite**

```bash
make test
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add redshift/provider.go
git commit -m "feat: add cluster_identifier and username to data_api schema"
```

---

## Task 4: Add provisioned cluster example and update docs template

**Files:**
- Create: `examples/provider/provider_using_redshift_data_api_provisioned.tf`
- Modify: `templates/index.md.tmpl`

- [ ] **Step 1: Create the provisioned cluster example file**

```hcl
provider "redshift" {
  database = var.redshift_database
  data_api {
    cluster_identifier = var.redshift_cluster_identifier
    username           = var.redshift_username
    region             = var.aws_region
  }
}
```

Save to `examples/provider/provider_using_redshift_data_api_provisioned.tf`.

- [ ] **Step 2: Reference the new example in `templates/index.md.tmpl`**

After the existing serverless example line:
```
{{ tffile "examples/provider/provider_using_redshift_data_api.tf" }}
```

Add:
```
### Authentication using Redshift Data API (provisioned cluster)

{{ tffile "examples/provider/provider_using_redshift_data_api_provisioned.tf" }}
```

- [ ] **Step 3: Run unit tests one final time**

```bash
make test
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add examples/provider/provider_using_redshift_data_api_provisioned.tf templates/index.md.tmpl
git commit -m "docs: add provisioned cluster Data API example"
```

---

## Self-Review Checklist

- [x] **Spec coverage:**
  - Schema: `workgroup_name` → Optional + ExactlyOneOf ✓ (Task 3)
  - Schema: `cluster_identifier` + `username` added ✓ (Task 3)
  - DSN builder for cluster ✓ (Task 2)
  - `getConfigFromDataApiResourceData` branches on cluster vs workgroup ✓ (Task 2)
  - `getConfigFromResourceData` detects cluster mode ✓ (Task 3)
  - Error when `cluster_identifier` set without `username` ✓ (Task 2, tested in Task 1)
  - Description updated to remove "serverless only" ✓ (Task 3)
  - Example file + docs template updated ✓ (Task 4)
- [x] **No placeholders:** All code blocks are complete
- [x] **Type consistency:** `newDataApiClusterConfig` defined in Task 2, called consistently
