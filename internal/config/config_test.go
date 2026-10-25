package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "trireme.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

const valid = `{"data_dir":"/d","doctrine_dir":"/doc","projects":[{"id":"p","repo":"/r","cap_usd":5,"seats":[{"id":"technical-lead"}]}]}`

func TestLoad_ValidFile_AppliesDefaults(t *testing.T) {
	inst, err := Load(write(t, valid))
	if err != nil {
		t.Fatal(err)
	}
	if inst.Listen != "127.0.0.1:8740" || inst.ClaudeBinary != "claude" || inst.BodyLabel == "" || inst.Projects[0].CompactAt != 200_000 {
		t.Fatalf("defaults not applied: %+v", inst)
	}
}

func TestLoad_InvalidFiles_ReturnErrors(t *testing.T) {
	cases := map[string]string{
		"not json":     `{`,
		"no data dir":  `{"doctrine_dir":"/doc"}`,
		"no seats":     `{"data_dir":"/d","doctrine_dir":"/doc","projects":[{"id":"p","repo":"/r","cap_usd":5}]}`,
		"no cap":       `{"data_dir":"/d","doctrine_dir":"/doc","projects":[{"id":"p","repo":"/r","seats":[{"id":"a"}]}]}`,
		"duplicate id": `{"data_dir":"/d","doctrine_dir":"/doc","projects":[{"id":"p","repo":"/r","cap_usd":5,"seats":[{"id":"a"}]},{"id":"p","repo":"/r","cap_usd":5,"seats":[{"id":"a"}]}]}`,
	}
	for name, body := range cases {
		if _, err := Load(write(t, body)); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Error("missing file: expected error")
	}
}

func writeRole(t *testing.T, dir, rel, body string) {
	t.Helper()
	path := filepath.Join(dir, "roles", rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCard_SharedAndSeatFiles_JoinsSharedFirst(t *testing.T) {
	dir := t.TempDir()
	writeRole(t, dir, "AGENTS.md", "# Every Seat")
	writeRole(t, dir, "qa-engineer/AGENTS.md", "# QA")
	card, err := Instance{DoctrineDir: dir}.Card("qa-engineer")
	if err != nil || !strings.HasPrefix(card, "# Every Seat") || !strings.HasSuffix(card, "# QA") {
		t.Fatalf("card %q err %v", card, err)
	}
}

func TestCard_MissingSeatOrSharedFile_Errors(t *testing.T) {
	dir := t.TempDir()
	writeRole(t, dir, "qa-engineer/AGENTS.md", "# QA")
	inst := Instance{DoctrineDir: dir}
	if _, err := inst.Card("qa-engineer"); err == nil {
		t.Fatal("missing shared file: expected error")
	}
	writeRole(t, dir, "AGENTS.md", "# Every Seat")
	if _, err := inst.Card("nobody"); err == nil {
		t.Fatal("missing seat file: expected error")
	}
}

func TestCard_PathEscapingRoles_Errors(t *testing.T) {
	dir := t.TempDir()
	writeRole(t, dir, "AGENTS.md", "# Every Seat")
	writeRole(t, dir, "../outside/AGENTS.md", "# Not a role")
	if card, err := (Instance{DoctrineDir: dir}).Card("../outside"); err == nil {
		t.Fatalf("read %q from outside roles/", card)
	}
}

func TestXOCard_OwnFileOnly_OmitsSharedFile(t *testing.T) {
	dir := t.TempDir()
	writeRole(t, dir, "AGENTS.md", "# Every Seat")
	writeRole(t, dir, "xo/AGENTS.md", "# XO")
	card, err := Instance{DoctrineDir: dir}.XOCard()
	if err != nil || card != "# XO" {
		t.Fatalf("card %q err %v", card, err)
	}
}
