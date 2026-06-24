package model_test

import (
	"testing"
	"time"

	"github.com/CyrusSE/agenthop/internal/model"
)

func TestOriginDigestStable(t *testing.T) {
	conv := &model.Conversation{
		Messages: []model.Message{
			{Role: model.RoleUser, Content: "hello", Timestamp: time.Unix(1, 0)},
			{Role: model.RoleAssistant, Content: "world", Timestamp: time.Unix(2, 0)},
		},
	}
	d1 := model.OriginDigest(conv)
	d2 := model.OriginDigest(conv)
	if d1 != d2 || len(d1) != 64 {
		t.Fatalf("digest unstable: %s %s", d1, d2)
	}
}

func TestSummaryShortID(t *testing.T) {
	s := model.Summary{ID: "01234567-89ab-cdef-0123-456789abcdef"}
	if s.ShortID() != "89abcdef" {
		t.Fatalf("short id = %q", s.ShortID())
	}
}
