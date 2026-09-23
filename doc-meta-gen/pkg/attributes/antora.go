package attributes

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type antoraDoc struct {
	Name     string `yaml:"name"`
	Asciidoc struct {
		Attributes map[string]any `yaml:"attributes"`
	} `yaml:"asciidoc"`
}

// LoadAntoraFile reads component name and asciidoc.attributes from an antora.yml file.
func LoadAntoraFile(path string) (name string, attrs map[string]string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read antora.yml %s: %w", path, err)
	}

	var doc antoraDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", nil, fmt.Errorf("failed to parse antora.yml %s: %w", path, err)
	}

	attrs = stringifyAttrs(doc.Asciidoc.Attributes)
	return doc.Name, attrs, nil
}

func stringifyAttrs(in map[string]any) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		if k == "" || v == nil {
			continue
		}
		out[k] = fmt.Sprint(v)
	}
	return out
}
