package main

import "testing"

func TestValidatePassword(t *testing.T) {
	ok := []string{"abcdef", "kataSandiContoh", "rahasia123", "p@ssw0rd!", string(make([]byte, 72))}
	for _, p := range ok {
		if err := validatePassword(p); err != nil {
			t.Errorf("password %d byte harus valid, dapat error: %v", len(p), err)
		}
	}
	bad := []string{"", "12345", "abcde", string(make([]byte, 73))}
	for _, p := range bad {
		if err := validatePassword(p); err == nil {
			t.Errorf("password %d byte harus ditolak", len(p))
		}
	}
}

func TestValidateUsername(t *testing.T) {
	ok := []string{"admin", "budi", "budi.santoso", "user_1", "a-b-c", "01234567890123456789012345678912"}
	for _, u := range ok {
		if err := validateUsername(u); err != nil {
			t.Errorf("username %q harus valid: %v", u, err)
		}
	}
	bad := []string{
		"", "ab",
		"0123456789012345678901234567890123",
		"bad name", "satu!", "a/b", "a\\b", "user@host", "emoji😀",
	}
	for _, u := range bad {
		if err := validateUsername(u); err == nil {
			t.Errorf("username %q harus ditolak", u)
		}
	}
}

func TestRoleLabel(t *testing.T) {
	if roleLabel(roleSuper) != "Super Admin" {
		t.Errorf("role super -> %q", roleLabel(roleSuper))
	}
	if roleLabel(roleAdmin) != "Admin" {
		t.Errorf("role admin -> %q", roleLabel(roleAdmin))
	}
}

func TestRoleWhitelist(t *testing.T) {
	for _, r := range []string{roleAdmin, roleSuper} {
		if r != roleAdmin && r != roleSuper {
			t.Errorf("role %q tidak dikenal", r)
		}
	}
	for _, r := range []string{"", "root", "user", "ADMIN", "super", "superadmin "} {
		if r == roleAdmin || r == roleSuper {
			t.Errorf("role %q seharusnya tidak lolos whitelist", r)
		}
	}
}
