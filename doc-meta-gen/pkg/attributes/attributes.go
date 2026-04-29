package attributes

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Store holds parsed AsciiDoc attributes for resolution
type Store struct {
	attributes map[string]string
	brands     []Brand
}

// Brand represents a product/brand entity from the attributes file
type Brand struct {
	Key    string
	Name   string
	Family string // "opensuse" or "suse"
}

// NewStore creates a new attribute store
func NewStore() *Store {
	return &Store{
		attributes: make(map[string]string),
		brands:     make([]Brand, 0),
	}
}

// LoadFromFiles loads one or more attribute/entity files into the store.
// Each file is parsed according to its extension:
//   - .ent  → XML entity declarations  (<!ENTITY key "value">)
//   - .adoc / anything else → AsciiDoc attribute syntax  (:key: value)
//
// The buildContext is applied once before any file is read so that
// conditional directives (ifndef/ifeval) work correctly across all files.
func (s *Store) LoadFromFiles(paths []string, buildContext map[string]string) error {
	// Seed with build context once
	for k, v := range buildContext {
		s.attributes[k] = v
	}
	for _, p := range paths {
		var err error
		if strings.HasSuffix(strings.ToLower(p), ".ent") {
			err = s.loadEntFile(p)
		} else {
			err = s.loadAdocFile(p)
		}
		if err != nil {
			return err
		}
	}
	s.resolveNestedAttributes()
	s.extractBrands()
	return nil
}

// LoadFromFile reads and parses a single attribute or entity file.
// Prefer LoadFromFiles when you have more than one file.
func (s *Store) LoadFromFile(path string, buildContext map[string]string) error {
	if path == "" {
		return nil
	}
	return s.LoadFromFiles([]string{path}, buildContext)
}

// loadEntFile parses a .ent XML entity declarations file.
// Handles both double-quoted and single-quoted values:
//
//	<!ENTITY productname   "SLES for SAP Applications">
//	<!ENTITY product-ga    '15'>
//
// Parameter entities (<!ENTITY % name SYSTEM "file.ent">) are ignored.
func (s *Store) loadEntFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to open entities file %s: %w", path, err)
	}
	// Match named entities with either double or single quoted values.
	// Group 1: entity name, Group 2: double-quoted value, Group 3: single-quoted value.
	entityRe := regexp.MustCompile(`<!ENTITY\s+([\w-]+)\s+(?:"([^"]*)"|'([^']*)')`)
	for _, m := range entityRe.FindAllSubmatch(data, -1) {
		value := string(m[2])
		if value == "" && m[3] != nil {
			value = string(m[3])
		}
		s.attributes[string(m[1])] = value
	}
	return nil
}

// loadAdocFile reads and parses an AsciiDoc attributes file.
func (s *Store) loadAdocFile(path string) error {
	if path == "" {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open attributes file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	attrRe := regexp.MustCompile(`^:([\w-]+):(?:\s+(.*))?$`)
	ifndefRe := regexp.MustCompile(`^ifndef::([\w-]+)\[\]$`)
	ifevalRe := regexp.MustCompile(`^ifeval::\["\{([\w-]+)\}" == "([^"]+)"\]$`)
	endifRe := regexp.MustCompile(`^endif::\[\]$`)

	inActiveBlock := true
	blockStack := []bool{}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Handle endif
		if endifRe.MatchString(line) {
			if len(blockStack) > 0 {
				inActiveBlock = blockStack[len(blockStack)-1]
				blockStack = blockStack[:len(blockStack)-1]
			}
			continue
		}

		// Handle ifndef
		if matches := ifndefRe.FindStringSubmatch(line); matches != nil {
			key := matches[1]
			blockStack = append(blockStack, inActiveBlock)
			_, exists := s.attributes[key]
			inActiveBlock = inActiveBlock && !exists
			continue
		}

		// Handle ifeval
		if matches := ifevalRe.FindStringSubmatch(line); matches != nil {
			key := matches[1]
			value := matches[2]
			blockStack = append(blockStack, inActiveBlock)
			isMatch := s.attributes[key] == value
			inActiveBlock = inActiveBlock && isMatch
			continue
		}

		if !inActiveBlock {
			continue
		}

		// Parse attribute definition
		if matches := attrRe.FindStringSubmatch(line); matches != nil {
			key := matches[1]
			value := ""
			if len(matches) > 2 {
				value = strings.TrimSpace(matches[2])
			}
			s.attributes[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading attributes file: %w", err)
	}

	return nil
}

// resolveNestedAttributes iteratively resolves attribute references
func (s *Store) resolveNestedAttributes() {
	maxIterations := 10
	for iteration := 0; iteration < maxIterations; iteration++ {
		changed := false
		for key, value := range s.attributes {
			if !strings.Contains(value, "{") {
				continue
			}

			newValue := value
			placeholderRe := regexp.MustCompile(`\{([\w-]+)\}`)
			matches := placeholderRe.FindAllStringSubmatch(value, -1)

			for _, match := range matches {
				placeholder := match[1]
				if replacement, exists := s.attributes[placeholder]; exists {
					newValue = strings.Replace(newValue, match[0], replacement, -1)
					changed = true
				}
			}

			s.attributes[key] = newValue
		}

		if !changed {
			break
		}
	}
}

// extractBrands identifies product/brand entities from attributes
func (s *Store) extractBrands() {
	for key, value := range s.attributes {
		if value == "" {
			continue
		}

		family := "suse"
		lowerValue := strings.ToLower(value)
		if strings.Contains(lowerValue, "opensuse") || strings.Contains(lowerValue, "leap") {
			family = "opensuse"
		}

		s.brands = append(s.brands, Brand{
			Key:    key,
			Name:   strings.TrimSpace(value),
			Family: family,
		})
	}
}

// Resolve replaces attribute references in text
func (s *Store) Resolve(text string) string {
	if !strings.Contains(text, "{") {
		return text
	}

	result := text
	maxIterations := 10

	for iteration := 0; iteration < maxIterations; iteration++ {
		temp := result
		for key, value := range s.attributes {
			placeholder := fmt.Sprintf("{%s}", key)
			temp = strings.ReplaceAll(temp, placeholder, value)
		}

		if temp == result {
			break
		}
		result = temp
	}

	// Remove any unresolved attributes
	placeholderRe := regexp.MustCompile(`\{[\w-]+\}`)
	result = placeholderRe.ReplaceAllString(result, "")

	return result
}

// ResolveXML replaces XML entity references (&key;) in text with their stored values.
// This is the DocBook counterpart to Resolve(), which handles {key} style references.
func (s *Store) ResolveXML(text string) string {
	if !strings.Contains(text, "&") {
		return text
	}

	result := text
	maxIterations := 10

	for range maxIterations {
		temp := result
		for key, value := range s.attributes {
			placeholder := fmt.Sprintf("&%s;", key)
			temp = strings.ReplaceAll(temp, placeholder, value)
		}
		if temp == result {
			break
		}
		result = temp
	}

	// Leave unresolved &entity; references alone — they may be structural XML.
	return result
}

// Get retrieves an attribute value
func (s *Store) Get(key string) (string, bool) {
	value, exists := s.attributes[key]
	return value, exists
}

// GetBrands returns all identified brands
func (s *Store) GetBrands() []Brand {
	return s.brands
}

// GetAll returns all attributes
func (s *Store) GetAll() map[string]string {
	return s.attributes
}
