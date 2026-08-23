package server

import (
	"testing"
	"time"
)

func TestGrowthSeriesFromEpochsBuildsDailyCumulativeTotals(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	points := growthSeriesFromEpochs(
		[]int64{0, time.Date(2026, 8, 9, 8, 0, 0, 0, time.UTC).Unix()},
		[]int64{time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC).Unix()}, 3, now,
	)
	if len(points) != 3 || points[0].TotalUsers != 1 || points[1].TotalUsers != 2 || points[2].TotalCharacters != 1 {
		t.Fatalf("unexpected cumulative totals: %+v", points)
	}
}

func TestActivityFromLastLoginsUsesRollingWindows(t *testing.T) {
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	activity := activityFromLastLogins([]int64{
		now.Add(-time.Hour).Unix(), now.Add(-48 * time.Hour).Unix(), now.Add(-10 * 24 * time.Hour).Unix(),
	}, now)
	if activity.Daily != 1 || activity.Weekly != 2 || activity.Monthly != 3 {
		t.Fatalf("unexpected activity: %+v", activity)
	}
}
