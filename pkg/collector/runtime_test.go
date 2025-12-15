package collector

import (
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector()
	assert.NotNil(t, c)
	assert.NotNil(t, c.tidbCollector)
	assert.NotNil(t, c.pdCollector)
	assert.NotNil(t, c.tikvCollector)
	assert.NotNil(t, c.tiflashCollector)
}

func TestCollector_Collect(t *testing.T) {
	tests := []struct {
		name      string
		endpoints types.ClusterEndpoints
		req       *CollectDataRequirements
		wantErr   bool
	}{
		{
			name: "nil requirements",
			endpoints: types.ClusterEndpoints{
				TiDBAddr: "127.0.0.1:4000",
			},
			req:     nil,
			wantErr: true, // Will fail because we can't actually connect
		},
		{
			name: "with requirements",
			endpoints: types.ClusterEndpoints{
				TiDBAddr: "127.0.0.1:4000",
			},
			req: &CollectDataRequirements{
				Components:          []string{"tidb"},
				NeedConfig:          true,
				NeedSystemVariables: true,
				NeedAllTikvNodes:    false,
			},
			wantErr: true, // Will fail because we can't actually connect
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCollector()
			_, err := c.Collect(tt.endpoints, tt.req)
			// We expect errors because we can't actually connect to a real cluster
			// But we can test that the function handles nil requirements correctly
			if tt.req == nil {
				// With nil req, it should create default requirements
				// The error will be from connection failure, not from nil handling
				assert.Error(t, err)
			} else {
				assert.Error(t, err) // Connection error expected
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		value string
		want  bool
	}{
		{
			name:  "empty slice",
			slice: []string{},
			value: "tidb",
			want:  false,
		},
		{
			name:  "contains value",
			slice: []string{"tidb", "pd", "tikv"},
			value: "tidb",
			want:  true,
		},
		{
			name:  "does not contain value",
			slice: []string{"tidb", "pd", "tikv"},
			value: "tiflash",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.value)
			assert.Equal(t, tt.want, result)
		})
	}
}
