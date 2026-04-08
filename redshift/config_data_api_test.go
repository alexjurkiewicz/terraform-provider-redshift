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
	_, err := newDataApiClusterConfig("my-cluster", "", "mydb", "us-east-1", 1)
	if err == nil {
		t.Fatal("expected error when username is empty, got nil")
	}
}

func TestBuildConnStrFromDataApiWorkgroupConfig_Unchanged(t *testing.T) {
	got := buildConnStrFromDataApiConfig("my-workgroup", "mydb", "ap-southeast-2")
	want := "workgroup(my-workgroup)/mydb?region=ap-southeast-2&transactionMode=non-transactional&requestMode=blocking"
	if got != want {
		t.Errorf("buildConnStrFromDataApiConfig() = %q, want %q", got, want)
	}
}
