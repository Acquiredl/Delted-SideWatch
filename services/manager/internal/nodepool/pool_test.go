package nodepool

import "testing"

func TestHealthToFloat(t *testing.T) {
	tests := []struct {
		name   string
		health HealthStatus
		want   float64
	}{
		{name: "healthy is 1", health: HealthHealthy, want: 1},
		{name: "unhealthy is 0", health: HealthUnhealthy, want: 0},
		{name: "syncing is 0", health: HealthSyncing, want: 0},
		{name: "unknown is 0", health: HealthUnknown, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := healthToFloat(tt.health)
			if got != tt.want {
				t.Errorf("healthToFloat(%q) = %f, want %f", tt.health, got, tt.want)
			}
		})
	}
}

func TestNodeStatusConstants(t *testing.T) {
	if StatusRunning != "running" {
		t.Errorf("StatusRunning = %q, want \"running\"", StatusRunning)
	}
	if StatusStopped != "stopped" {
		t.Errorf("StatusStopped = %q, want \"stopped\"", StatusStopped)
	}
	if HealthHealthy != "healthy" {
		t.Errorf("HealthHealthy = %q, want \"healthy\"", HealthHealthy)
	}
	if HealthUnhealthy != "unhealthy" {
		t.Errorf("HealthUnhealthy = %q, want \"unhealthy\"", HealthUnhealthy)
	}
}

func TestXMRigConfFields(t *testing.T) {
	conf := XMRigConf{
		URL:  "sidewatch.example.com:3333",
		User: "4ABC...",
		Pass: "x",
	}
	if conf.URL == "" {
		t.Error("XMRigConf.URL is empty")
	}
	if conf.User == "" {
		t.Error("XMRigConf.User is empty")
	}
	if conf.Pass != "x" {
		t.Errorf("XMRigConf.Pass = %q, want \"x\"", conf.Pass)
	}
}

func TestNodeSummaryDefaults(t *testing.T) {
	s := NodeSummary{
		Name:      "SideWatch Mini",
		Sidechain: "mini",
		Status:    HealthUnknown,
	}
	if s.Hashrate != nil {
		t.Error("Hashrate should be nil by default")
	}
	if s.Miners != nil {
		t.Error("Miners should be nil by default")
	}
	if s.Peers != nil {
		t.Error("Peers should be nil by default")
	}
}
