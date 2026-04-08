package redshift

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	_ "github.com/mmichaelb/redshift-data-sql-driver"
)

const redshiftDataDriverName = "redshift-data"

func NewDataApiConfig(workgroupName, database, awsRegion string, maxConns int) *Config {
	connStr := buildConnStrFromDataApiConfig(workgroupName, database, awsRegion)
	return NewConfig(redshiftDataDriverName, connStr, database, maxConns)
}

func buildConnStrFromDataApiConfig(workgroupName, database, awsRegion string) string {
	return fmt.Sprintf(
		"workgroup(%s)/%s?region=%s&transactionMode=non-transactional&requestMode=blocking",
		workgroupName, database, awsRegion,
	)
}

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
