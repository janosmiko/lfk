package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// Union mode: identical namespace+name can exist in multiple clusters, so the
// carry-over lookups must key by cluster too — otherwise one cluster's carried
// columns overwrite another's (CodeRabbit review on PR #422).

func TestCarryOverMetricsColumnsFrom_UnionClustersDontCollide(t *testing.T) {
	oldItems := []model.Item{
		{
			Name: "pod-a", Namespace: "ns-1", ClusterName: "cluster-1",
			Columns: []model.KeyValue{{Key: "CPU", Value: "100m"}},
		},
		{
			Name: "pod-a", Namespace: "ns-1", ClusterName: "cluster-2",
			Columns: []model.KeyValue{{Key: "CPU", Value: "900m"}},
		},
	}
	newItems := []model.Item{
		{Name: "pod-a", Namespace: "ns-1", ClusterName: "cluster-1"},
		{Name: "pod-a", Namespace: "ns-1", ClusterName: "cluster-2"},
	}
	carryOverMetricsColumnsFrom(oldItems, newItems)

	assert.Equal(t, []model.KeyValue{{Key: "CPU", Value: "100m"}}, newItems[0].Columns,
		"cluster-1 row must keep cluster-1 metrics")
	assert.Equal(t, []model.KeyValue{{Key: "CPU", Value: "900m"}}, newItems[1].Columns,
		"cluster-2 row must keep cluster-2 metrics")
}

func TestCarryOverServiceEndpointColumnsFrom_UnionClustersDontCollide(t *testing.T) {
	oldItems := []model.Item{
		{
			Name: "svc", Namespace: "ns-1", ClusterName: "cluster-1",
			Columns: []model.KeyValue{{Key: "Backing Endpoints", Value: "2"}},
		},
		{
			Name: "svc", Namespace: "ns-1", ClusterName: "cluster-2",
			Columns: []model.KeyValue{{Key: "Backing Endpoints", Value: "7"}},
		},
	}
	newItems := []model.Item{
		{Name: "svc", Namespace: "ns-1", ClusterName: "cluster-1"},
		{Name: "svc", Namespace: "ns-1", ClusterName: "cluster-2"},
	}
	carryOverServiceEndpointColumnsFrom(oldItems, newItems)

	assert.Equal(t, []model.KeyValue{{Key: "Backing Endpoints", Value: "2"}}, newItems[0].Columns,
		"cluster-1 row must keep cluster-1 endpoints")
	assert.Equal(t, []model.KeyValue{{Key: "Backing Endpoints", Value: "7"}}, newItems[1].Columns,
		"cluster-2 row must keep cluster-2 endpoints")
}
