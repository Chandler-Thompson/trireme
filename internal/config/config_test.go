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

func TestCard_ExistingAndMissing_ReadsOrErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "roles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roles", "qa-engineer.md"), []byte("# QA"), 0o600); err != nil {
		t.Fatal(err)
	}
	inst := Instance{DoctrineDir: dir}
	if card, err := inst.Card("qa-engineer"); err != nil || !strings.HasPrefix(card, "# QA") {
		t.Fatalf("card %q err %v", card, err)
	}
	if _, err := inst.Card("nobody"); err == nil {
		t.Fatal("expected error")
	}
}
