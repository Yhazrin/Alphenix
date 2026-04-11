package handler

import (
	"net"
	"testing"

	db "github.com/multica-ai/alphenix/server/pkg/db/generated"
)

// --- roleAllowed ---

func TestRoleAllowed_Match(t *testing.T) {
	if !roleAllowed("owner", "owner", "admin") {
		t.Error("should match when role is in list")
	}
}

func TestRoleAllowed_NoMatch(t *testing.T) {
	if roleAllowed("member", "owner", "admin") {
		t.Error("should not match when role is not in list")
	}
}

func TestRoleAllowed_EmptyList(t *testing.T) {
	if roleAllowed("owner") {
		t.Error("empty roles list should never match")
	}
}

// --- countOwners ---

func TestCountOwners_Multiple(t *testing.T) {
	members := []db.Member{
		{Role: "owner"},
		{Role: "member"},
		{Role: "owner"},
		{Role: "admin"},
	}
	if got := countOwners(members); got != 2 {
		t.Errorf("countOwners() = %d, want 2", got)
	}
}

func TestCountOwners_None(t *testing.T) {
	members := []db.Member{
		{Role: "member"},
		{Role: "admin"},
	}
	if got := countOwners(members); got != 0 {
		t.Errorf("countOwners() = %d, want 0", got)
	}
}

func TestCountOwners_Empty(t *testing.T) {
	if got := countOwners(nil); got != 0 {
		t.Errorf("nil members should return 0, got %d", got)
	}
}

// --- slugifyWorkspacePart ---

func TestSlugifyWorkspacePart_Basic(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"  spaces  ", "spaces"},
		{"already-slug", "already-slug"},
		{"Special!@#Chars", "special-chars"},
		{"123 numbers 456", "123-numbers-456"},
		{"", ""},
		{"---", ""},
		{"a---b", "a-b"},
	}

	for _, tt := range tests {
		got := slugifyWorkspacePart(tt.input)
		if got != tt.want {
			t.Errorf("slugifyWorkspacePart(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- normalizeMemberRole ---

func TestNormalizeMemberRole_ValidRoles(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{"owner", "owner", true},
		{"admin", "admin", true},
		{"member", "member", true},
		{"", "member", true}, // empty defaults to member
		{"  owner  ", "owner", true},
		{"OWNER", "", false},
		{"guest", "", false},
		{"moderator", "", false},
	}

	for _, tt := range tests {
		got, ok := normalizeMemberRole(tt.input)
		if got != tt.want || ok != tt.ok {
			t.Errorf("normalizeMemberRole(%q) = (%q, %v), want (%q, %v)",
				tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

// --- isRestrictedIP ---

func TestIsRestrictedIP_Loopback(t *testing.T) {
	if !isRestrictedIP(net.ParseIP("127.0.0.1")) {
		t.Error("127.0.0.1 should be restricted")
	}
}

func TestIsRestrictedIP_Private10(t *testing.T) {
	if !isRestrictedIP(net.ParseIP("10.0.0.1")) {
		t.Error("10.0.0.1 should be restricted")
	}
}

func TestIsRestrictedIP_Private172(t *testing.T) {
	if !isRestrictedIP(net.ParseIP("172.16.0.1")) {
		t.Error("172.16.0.1 should be restricted")
	}
	if !isRestrictedIP(net.ParseIP("172.31.255.255")) {
		t.Error("172.31.255.255 should be restricted")
	}
	if isRestrictedIP(net.ParseIP("172.15.255.255")) {
		t.Error("172.15.x should NOT be restricted")
	}
	if isRestrictedIP(net.ParseIP("172.32.0.1")) {
		t.Error("172.32.x should NOT be restricted")
	}
}

func TestIsRestrictedIP_Private192_168(t *testing.T) {
	if !isRestrictedIP(net.ParseIP("192.168.1.1")) {
		t.Error("192.168.1.1 should be restricted")
	}
}

func TestIsRestrictedIP_CGNAT(t *testing.T) {
	if !isRestrictedIP(net.ParseIP("100.64.0.1")) {
		t.Error("100.64.0.1 (CGNAT) should be restricted")
	}
	if !isRestrictedIP(net.ParseIP("100.127.255.255")) {
		t.Error("100.127.255.255 (CGNAT) should be restricted")
	}
	if isRestrictedIP(net.ParseIP("100.63.255.255")) {
		t.Error("100.63.x should NOT be restricted")
	}
}

func TestIsRestrictedIP_Public(t *testing.T) {
	if isRestrictedIP(net.ParseIP("8.8.8.8")) {
		t.Error("8.8.8.8 should NOT be restricted")
	}
	if isRestrictedIP(net.ParseIP("1.1.1.1")) {
		t.Error("1.1.1.1 should NOT be restricted")
	}
}

func TestIsRestrictedIP_IPv6Local(t *testing.T) {
	if !isRestrictedIP(net.ParseIP("fc00::1")) {
		t.Error("fc00::1 (ULA) should be restricted")
	}
	if !isRestrictedIP(net.ParseIP("fd00::1")) {
		t.Error("fd00::1 (ULA) should be restricted")
	}
}

func TestIsRestrictedIP_IPv6Public(t *testing.T) {
	if isRestrictedIP(net.ParseIP("2001:4860:4860::8888")) {
		t.Error("public IPv6 should NOT be restricted")
	}
}
