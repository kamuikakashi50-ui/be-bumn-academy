package service

import (
	"testing"
	"time"
)

// Uji jeda pengingat berkala (Fase 2 §4.5) pada bagian murni: kapan jeda dianggap
// masih aktif. Kasus "belum pernah dikirim" tidak boleh memblokir pengiriman, dan
// tepat 14 hari sudah boleh dikirim lagi.

func TestBroadcastCooldownActive(t *testing.T) {
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	window := 14 * 24 * time.Hour

	at := func(offset time.Duration) *time.Time {
		value := now.Add(offset)
		return &value
	}

	cases := []struct {
		name string
		last *time.Time
		want bool
	}{
		{name: "belum pernah dikirim", last: nil, want: false},
		{name: "5 hari lalu", last: at(-5 * 24 * time.Hour), want: true},
		{name: "13 hari 23 jam lalu", last: at(-(window - time.Hour)), want: true},
		{name: "tepat 14 hari lalu", last: at(-window), want: false},
		{name: "20 hari lalu", last: at(-20 * 24 * time.Hour), want: false},
	}

	for _, tc := range cases {
		if got := broadcastCooldownActive(tc.last, now, window); got != tc.want {
			t.Errorf("%s: broadcastCooldownActive = %v, harap %v", tc.name, got, tc.want)
		}
	}
}

func TestBroadcastReminderAudienceTerdaftar(t *testing.T) {
	// Audience pengingat tracer study harus dibatasi jeda; audience lain tidak.
	if _, ok := broadcastReminderAudiences["outcome_pending"]; !ok {
		t.Error("audience outcome_pending harus dibatasi jeda 14 hari")
	}
	for _, audience := range []string{"all", "enrolled", "paid", "pending"} {
		if _, ok := broadcastReminderAudiences[audience]; ok {
			t.Errorf("audience %q tidak seharusnya dibatasi jeda", audience)
		}
	}
}
