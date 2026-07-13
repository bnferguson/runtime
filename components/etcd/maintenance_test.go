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
			name:   "near quota with reclaimable bloat: live data under high-water",
			dbSize: 7 * gib, dbSizeInUse: 5 * gib, quota: quota, // high-water 6.4 GiB
			want: actionReclaim,
		},
		{
			name:   "near quota but mostly live data: defrag cannot help, warn",
			dbSize: 7 * gib, dbSizeInUse: 6900 * mib, quota: quota, // in-use 6.7 GiB > 6.4 GiB high-water
			want: actionWarnFull,
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
	const quota = 10 * gib // 80% high-water = 8 GiB exactly

	// Strictly above the high-water, with live data under it, reclaims.
	if got := decideMaintenance(8*gib+1, 4*gib, quota, false); got != actionReclaim {
		t.Errorf("just above high-water with reclaimable bloat: got %d, want actionReclaim", got)
	}
	// Exactly at the high-water (not strictly above) does not trigger the near-quota path.
	if got := decideMaintenance(8*gib, 4*gib, quota, false); got == actionReclaim || got == actionWarnFull {
		t.Errorf("exactly at high-water should not trigger near-quota path, got %d", got)
	}
	// Above the high-water but live data also above it warns instead of thrashing.
	if got := decideMaintenance(9*gib, 8*gib+1, quota, false); got != actionWarnFull {
		t.Errorf("near quota, live data over high-water: got %d, want actionWarnFull", got)
	}
}
