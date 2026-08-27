package session

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeSkill writes a skill (name + files, where files maps a relative path
// inside the skill dir to its content) under dir/skills/, creating the dirs.
// It is the skills counterpart of writeAgent — skills are directories, so a
// skill is a whole file tree, not a single file.
func writeSkill(t *testing.T, dir, name string, files map[string]string) {
	t.Helper()
	skillDir := filepath.Join(dir, "skills", name)
	for rel, content := range files {
		p := filepath.Join(skillDir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestListSkills — only immediate subdirectories carrying a SKILL.md count as
// skills; files, non-skill dirs, and dirs without SKILL.md are ignored, and
// the result is sorted by name.
func TestListSkills(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n"})
	writeSkill(t, dir, "beta", map[string]string{
		"SKILL.md":      "---\nname: beta\n---\n",
		"helper.py":     "print('hi')",
		"sub/notes.txt": "support file",
	})
	// A directory that is not a skill (no SKILL.md) and a stray file.
	if err := os.MkdirAll(filepath.Join(dir, "skills", "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "todo.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A nested SKILL.md (inside a non-skill dir) must not be picked up as a
	// skill itself — only immediate subdirectories are enumerated.
	if err := os.MkdirAll(filepath.Join(dir, "skills", "notes", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "notes", "sub", "SKILL.md"), []byte("---\nname: nested\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := listSkills(filepath.Join(dir, "skills"))
	if err != nil {
		t.Fatalf("listSkills: %v", err)
	}
	want := []SkillDir{
		{Name: "alpha", Dir: filepath.Join(dir, "skills", "alpha")},
		{Name: "beta", Dir: filepath.Join(dir, "skills", "beta")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("listSkills = %+v, want %+v", got, want)
	}
}

// TestListSkillsMissingAndEmpty — a missing skills dir and one with no skill
// folders both yield an empty slice, not an error.
func TestListSkillsMissingAndEmpty(t *testing.T) {
	dir := t.TempDir()
	if got, err := listSkills(filepath.Join(dir, "skills")); err != nil || len(got) != 0 {
		t.Errorf("missing dir: got (%+v, %v), want empty, nil", got, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := listSkills(filepath.Join(dir, "skills")); err != nil || len(got) != 0 {
		t.Errorf("empty dir: got (%+v, %v), want empty, nil", got, err)
	}
}

// TestStageGlobalSkills — every skill under the source dir is copied whole
// (SKILL.md plus support files, including nested ones) into a fresh temp dir,
// with one directory per skill; the source tree is never modified.
func TestStageGlobalSkills(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n"})
	writeSkill(t, dir, "beta", map[string]string{
		"SKILL.md":      "---\nname: beta\n---\n",
		"helper.py":     "print('hi')",
		"sub/notes.txt": "support file",
	})

	tmp, ok, err := stageGlobalSkills(filepath.Join(dir, "skills"))
	if err != nil {
		t.Fatalf("stageGlobalSkills: %v", err)
	}
	defer os.RemoveAll(tmp)
	if !ok {
		t.Fatal("stageGlobalSkills reported no skills for a non-empty skills/")
	}

	for name, files := range map[string][]string{
		"alpha": {"SKILL.md"},
		"beta":  {"SKILL.md", "helper.py", filepath.Join("sub", "notes.txt")},
	} {
		for _, rel := range files {
			staged := filepath.Join(tmp, name, rel)
			if _, err := os.Stat(staged); err != nil {
				t.Errorf("staged %s missing: %v", staged, err)
			}
		}
	}
	// No extra top-level entries beyond the skills themselves.
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("staged dir has %d entries, want 2 (alpha, beta)", len(entries))
	}

	// The host source tree is untouched — content and structure preserved.
	data, err := os.ReadFile(filepath.Join(dir, "skills", "beta", "SKILL.md"))
	if err != nil || !strings.Contains(string(data), "name: beta") {
		t.Errorf("source skill was modified: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "skills", "beta", "sub", "notes.txt")); err != nil {
		t.Errorf("source support file missing: %v", err)
	}
}

// TestStageGlobalSkillsNoSkills — a missing skills dir and one with no skill
// folders both yield ("", false, nil): nothing staged, no temp dir leaked.
func TestStageGlobalSkillsNoSkills(t *testing.T) {
	dir := t.TempDir()
	if tmp, ok, err := stageGlobalSkills(filepath.Join(dir, "skills")); err != nil || ok || tmp != "" {
		t.Errorf("missing dir: got (%q, %v, %v), want (\"\", false, nil)", tmp, ok, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if tmp, ok, err := stageGlobalSkills(filepath.Join(dir, "skills")); err != nil || ok || tmp != "" {
		t.Errorf("empty dir: got (%q, %v, %v), want (\"\", false, nil)", tmp, ok, err)
	}
}

// TestStageGlobalSkillsSkipsNonSkills — only skill folders are staged; a
// non-skill directory and a stray file in skills/ are not copied.
func TestStageGlobalSkillsSkipsNonSkills(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "alpha", map[string]string{"SKILL.md": "---\nname: alpha\n---\n"})
	if err := os.MkdirAll(filepath.Join(dir, "skills", "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "notes", "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "todo.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	tmp, ok, err := stageGlobalSkills(filepath.Join(dir, "skills"))
	if err != nil {
		t.Fatalf("stageGlobalSkills: %v", err)
	}
	defer os.RemoveAll(tmp)
	if !ok {
		t.Fatal("stageGlobalSkills reported no skills for a non-empty skills/")
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "alpha" {
		t.Errorf("staged dir = %v, want only alpha", entries)
	}
}
