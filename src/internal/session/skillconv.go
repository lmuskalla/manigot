package session

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/lmuskalla/manigot/internal/fs"
)

// SkillDir is one discovered skill: the skill's name (its directory name, the
// identifier both CLIs use to address the skill) and the path to that
// directory (which holds the SKILL.md plus any support files).
type SkillDir struct {
	// Name is the skill's directory name — the name both CLIs derive the
	// skill's identifier from.
	Name string

	// Dir is the absolute path to the skill's directory.
	Dir string
}

// listSkills enumerates the skill folders under srcDir — every immediate
// subdirectory that contains a SKILL.md — returning them as <name>/ → dir
// pairs sorted by name. A missing srcDir, or one with no skill folders,
// yields an empty slice, not an error (skills are optional; every caller
// skips cleanly when there is nothing to deliver).
func listSkills(srcDir string) ([]SkillDir, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var skills []SkillDir
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(srcDir, e.Name())
		if !fs.IsFile(filepath.Join(dir, "SKILL.md")) {
			continue
		}
		skills = append(skills, SkillDir{Name: e.Name(), Dir: dir})
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

// stageGlobalSkills copies every skill under srcDir into a fresh temp
// directory — the directory-level counterpart of convertAgents (which stages
// single converted agent files). Skills need no frontmatter conversion: both
// CLIs read SKILL.md and its frontmatter (name:, description:, ...) natively,
// so the copy is a plain recursive copy preserving each skill's directory and
// its support files. The staged dir is meant for a read-only container mount
// (see BuildDockerInvocation's global-skills block) and is removed by the
// caller via the invocation's Cleanup hook — the host's skills/ source tree
// is never modified.
//
// It returns the temp dir path and true when srcDir contained at least one
// skill; ("", false, nil) when srcDir is missing or has no skills (nothing to
// stage); and ("", false, err) on any failure, after removing any temp dir it
// already created.
func stageGlobalSkills(srcDir string) (string, bool, error) {
	skills, err := listSkills(srcDir)
	if err != nil {
		return "", false, err
	}
	if len(skills) == 0 {
		return "", false, nil
	}
	tmp, err := os.MkdirTemp("", "manigot-skills-*")
	if err != nil {
		return "", false, err
	}
	for _, s := range skills {
		if err := copyDir(filepath.Join(tmp, s.Name), s.Dir); err != nil {
			os.RemoveAll(tmp)
			return "", false, err
		}
	}
	return tmp, true, nil
}

// copyDir recursively copies the directory src to dst, recreating the
// directory structure and copying file contents. It is the directory-level
// equivalent of convertAgents' single-file writes — a skill is a directory
// (SKILL.md plus optional support files), so the whole tree must move.
// Symlinked entries are copied by content (the link target's data), not
// recreated as links, so a staged/host copy can never carry a symlink that
// points outside the copied tree.
func copyDir(dst, src string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
