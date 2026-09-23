package discovery

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/scribe/doc-meta-gen/pkg/attributes"
)

// VersionedFile is a documentation page that belongs to an Antora component version.
type VersionedFile struct {
	Path    string // absolute file path
	RelPath string // path relative to the antora.yml directory
	Version string // version directory basename (v1.3.0, latest, next, Dev, …)
	Name    string // Antora component name
}

// ComponentGroup is one Antora component with its chosen source version and the rest.
type ComponentGroup struct {
	Name          string
	SourceVersion string
	SourceFiles   []VersionedFile
	OtherFiles    []VersionedFile
}

type antoraMeta struct {
	name  string
	attrs map[string]string
}

// GroupByComponent splits files into Antora component groups and leftover ungrouped paths.
// The source version is the highest semantic version when any exist; otherwise next, then
// dev, then latest.
func GroupByComponent(files []string, root string) (groups []ComponentGroup, ungrouped []string) {
	metaCache := map[string]antoraMeta{}
	byName := map[string][]VersionedFile{}

	for _, path := range files {
		antoraPath := FindAntoraYml(path, root)
		if antoraPath == "" {
			ungrouped = append(ungrouped, path)
			continue
		}

		meta, ok := metaCache[antoraPath]
		if !ok {
			name, attrs, err := attributes.LoadAntoraFile(antoraPath)
			if err != nil {
				log.Printf("WARNING: %v — treating files under this component as ungrouped", err)
				metaCache[antoraPath] = antoraMeta{}
				ungrouped = append(ungrouped, path)
				continue
			}
			if name == "" {
				name = filepath.Base(filepath.Dir(antoraPath))
			}
			meta = antoraMeta{name: name, attrs: attrs}
			metaCache[antoraPath] = meta
		}
		if meta.name == "" {
			ungrouped = append(ungrouped, path)
			continue
		}

		versionDir := filepath.Dir(antoraPath)
		rel, err := filepath.Rel(versionDir, path)
		if err != nil {
			ungrouped = append(ungrouped, path)
			continue
		}

		byName[meta.name] = append(byName[meta.name], VersionedFile{
			Path:    path,
			RelPath: filepath.ToSlash(rel),
			Version: filepath.Base(versionDir),
			Name:    meta.name,
		})
	}

	names := make([]string, 0, len(byName))
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		filesForName := byName[name]
		versions := uniqueVersions(filesForName)
		source := PickSourceVersion(versions)

		group := ComponentGroup{Name: name, SourceVersion: source}
		for _, f := range filesForName {
			if f.Version == source {
				group.SourceFiles = append(group.SourceFiles, f)
			} else {
				group.OtherFiles = append(group.OtherFiles, f)
			}
		}
		groups = append(groups, group)

		others := make([]string, 0, len(versions)-1)
		for _, v := range versions {
			if v != source {
				others = append(others, v)
			}
		}
		if len(others) > 0 {
			log.Printf("Component %s: generate from %s, copy to %s", name, source, strings.Join(others, ", "))
		} else {
			log.Printf("Component %s: single version %s", name, source)
		}
	}

	return groups, ungrouped
}

// FindAntoraYml walks up from path until root looking for antora.yml.
func FindAntoraYml(path, root string) string {
	dir := path
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		dir = filepath.Dir(path)
	}

	root = filepath.Clean(root)
	for {
		candidate := filepath.Join(dir, "antora.yml")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		if dir == root || dir == string(filepath.Separator) || dir == "." {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// AntoraAttributes returns asciidoc.attributes from the nearest antora.yml.
func AntoraAttributes(path, root string) map[string]string {
	antoraPath := FindAntoraYml(path, root)
	if antoraPath == "" {
		return nil
	}
	_, attrs, err := attributes.LoadAntoraFile(antoraPath)
	if err != nil {
		log.Printf("WARNING: %v", err)
		return nil
	}
	return attrs
}

// PickSourceVersion chooses the version to generate from.
// Semantic versions win (highest). If none exist: next, then dev, then latest.
func PickSourceVersion(labels []string) string {
	if len(labels) == 0 {
		return ""
	}

	type semver struct {
		label string
		maj   int
		min   int
		pat   int
	}

	var semvers []semver
	var next, dev, latest string

	for _, label := range labels {
		if maj, min, pat, ok := parseSemver(label); ok {
			semvers = append(semvers, semver{label: label, maj: maj, min: min, pat: pat})
			continue
		}
		switch strings.ToLower(label) {
		case "next":
			next = label
		case "dev":
			dev = label
		case "latest":
			latest = label
		}
	}

	if len(semvers) > 0 {
		sort.Slice(semvers, func(i, j int) bool {
			a, b := semvers[i], semvers[j]
			if a.maj != b.maj {
				return a.maj > b.maj
			}
			if a.min != b.min {
				return a.min > b.min
			}
			return a.pat > b.pat
		})
		return semvers[0].label
	}
	if next != "" {
		return next
	}
	if dev != "" {
		return dev
	}
	if latest != "" {
		return latest
	}
	sort.Strings(labels)
	return labels[len(labels)-1]
}

func parseSemver(label string) (maj, min, pat int, ok bool) {
	s := strings.TrimSpace(label)
	lower := strings.ToLower(s)
	switch {
	case strings.HasPrefix(lower, "version-"):
		s = s[len("version-"):]
	case strings.HasPrefix(lower, "version"):
		s = s[len("version"):]
	case strings.HasPrefix(lower, "v") && len(s) > 1 && s[1] >= '0' && s[1] <= '9':
		s = s[1:]
	}

	parts := strings.Split(s, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, 0, 0, false
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return 0, 0, 0, false
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], true
}

func uniqueVersions(files []VersionedFile) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, f := range files {
		if _, ok := seen[f.Version]; ok {
			continue
		}
		seen[f.Version] = struct{}{}
		out = append(out, f.Version)
	}
	return out
}
