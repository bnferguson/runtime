package etcd

import "testing"

func TestDecideMaintenance(t *testing.T) {
	const quota = 8 * gib // high-water at 0.80 → 6.4 GiB

	tests := []struct {
		name                       string
		dbSize, dbSizeInUse, quota int64
		noSpace                    bool
		want                       maintenanceAction
	}{
		{
			name:   "healthy: low bloat, well under quota",
			dbSize: 1 * gib, dbSizeInUse: 900 * mib, quota: quota,
			want: actionNone,
		},
		{
			name:   "bloated: ratio over 2x, under high-water",
			dbSize: 1 * gib, dbSizeInUse: 400 * mib, quota: quota,
			want: actionDefrag,
		},
		{
			name:   "near quota: above 80% high-water, low bloat",
			dbSize: 7 * gib, dbSizeInUse: 6900 * mib, quota: quota,
			want: actionReclaim,
		},
		{
			name:   "near quota takes precedence over bloat ratio",
			dbSize: 7 * gib, dbSizeInUse: 1 * gib, quota: quota,
			want: actionReclaim,
		},
		{
			name:   "NOSPACE takes precedence over everything",
			dbSize: 7 * gib, dbSizeInUse: 1 * gib, quota: quota, noSpace: true,
			want: actionRecover,
		},
		{
			name:   "NOSPACE with an otherwise-healthy-looking size still recovers",
			dbSize: 1 * gib, dbSizeInUse: 1 * gib, quota: quota, noSpace: true,
			want: actionRecover,
		},
		{
			name:   "no quota known: falls back to bloat ratio only (near-quota disabled)",
			dbSize: 7 * gib, dbSizeInUse: 6900 * mib, quota: 0,
			want: actionNone,
		},
		{
			name:   "no quota known but bloated: defrags",
			dbSize: 7 * gib, dbSizeInUse: 1 * gib, quota: 0,
			want: actionDefrag,
		},
		{
			name:   "zero dbSizeInUse never divides / never defrags on ratio",
			dbSize: 1 * gib, dbSizeInUse: 0, quota: quota,
			want: actionNone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := decideMaintenance(tc.dbSize, tc.dbSizeInUse, tc.quota, tc.noSpace)
			if got != tc.want {
				t.Errorf("decideMaintenance(%d,%d,%d,%v) = %d, want %d",
					tc.dbSize, tc.dbSizeInUse, tc.quota, tc.noSpace, got, tc.want)
			}
		})
	}
}

func TestDecideMaintenanceHighWaterBoundary(t *testing.T) {
	const quota = 10 * gib // 80% = 8 GiB exactly

	// Strictly above the high-water reclaims; at/below does not (assuming low bloat).
	if got := decideMaintenance(8*gib+1, 8*gib, quota, false); got != actionReclaim {
		t.Errorf("just above high-water: got %d, want actionReclaim", got)
	}
	if got := decideMaintenance(8*gib, 7*gib+900*mib, quota, false); got == actionReclaim {
		t.Errorf("exactly at high-water should not reclaim, got actionReclaim")
	}
}
