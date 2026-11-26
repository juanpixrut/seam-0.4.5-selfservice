package config

import "testing"

func TestFeatureConfig_DeveloperSyncSecondsBounds(t *testing.T) {
	for _, c := range []struct {
		n    int
		okay bool
	}{{0, true}, {2, true}, {15, true}, {3600, true}, {1, false}, {-1, false}, {-86400, false}, {3601, false}, {1 << 40, false}} {
		err := FeatureConfig{DeveloperSyncSeconds: c.n}.validate()
		if (err == nil) != c.okay {
			t.Errorf("%d seconds: error %v, want ok=%v", c.n, err, c.okay)
		}
	}
}
