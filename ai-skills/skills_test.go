package skills

import (
	"io/fs"
	"path"
	"regexp"
	"strings"
	"testing"
)

func TestSkillInventory(t *testing.T) {
	tree, err := Tree("work-plugin-builder")
	if err != nil {
		t.Fatal(err)
	}
	b, err := fs.ReadFile(tree, "SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.HasPrefix(s, "---\nname: work-plugin-builder\ndescription:") {
		t.Fatal("frontmatter")
	}
	for _, name := range []string{"marketplace-catalog", "plugin-manifest", "component-roles", "validate-and-publish"} {
		if !strings.Contains(s, "references/"+name+".md") {
			t.Fatal("missing reference", name)
		}
	}
	if _, err = Tree("../escape"); err == nil {
		t.Fatal("unknown name")
	}
	if _, err = Digest("work-plugin-builder"); err != nil {
		t.Fatal(err)
	}
}

func TestAllPayloadLinksAndFrontmatter(t *testing.T) {
	tree, err := Tree("work-plugin-builder")
	if err != nil {
		t.Fatal(err)
	}
	err = fs.WalkDir(tree, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		b, err := fs.ReadFile(tree, p)
		if err != nil {
			return err
		}
		for _, m := range regexp.MustCompile(`\]\(([^)#]+)(?:#[^)]*)?\)`).FindAllStringSubmatch(string(b), -1) {
			link := m[1]
			if strings.Contains(link, "://") {
				continue
			}
			dest := path.Clean(path.Join(path.Dir(p), link))
			if _, err = fs.Stat(tree, dest); err != nil {
				t.Errorf("broken link %s -> %s", p, link)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := fs.ReadFile(tree, "SKILL.md")
	s := string(b)
	front := strings.Split(s, "---")[1]
	if len(strings.FieldsFunc(strings.TrimSpace(front), func(r rune) bool { return r == '\n' })) != 2 {
		t.Fatal("vendor metadata in frontmatter")
	}
	for _, bad := range []string{"$ARGUMENTS", "openai.yaml", "Claude", "Codex"} {
		if strings.Contains(s, bad) {
			t.Fatal("vendor-specific body", bad)
		}
	}
}
