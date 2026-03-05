package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/scribe/doc-meta-gen/internal/ai"
	"github.com/scribe/doc-meta-gen/internal/discovery"
	"github.com/scribe/doc-meta-gen/internal/models"
	"github.com/scribe/doc-meta-gen/internal/processor"
	"github.com/scribe/doc-meta-gen/internal/providers"
	"github.com/scribe/doc-meta-gen/internal/providers/asciidoc"
	"github.com/scribe/doc-meta-gen/pkg/attributes"
)

func main() {
	// Parse command-line flags
	config := parseFlags()

	// Validate root directory
	if _, err := os.Stat(config.RootDir); os.IsNotExist(err) {
		log.Fatalf("ERROR: Root directory does not exist: %s", config.RootDir)
	}

	log.Printf("Starting doc-meta-gen")
	log.Printf("Root directory: %s", config.RootDir)
	log.Printf("Model: %s", config.ModelName)
	log.Printf("Ollama URL: %s", config.OllamaURL)

	// Load attributes
	attrStore := attributes.NewStore()
	if config.AttributesFile != "" {
		log.Printf("Loading attributes from: %s", config.AttributesFile)
		if err := attrStore.LoadFromFile(config.AttributesFile, config.BuildAttributes); err != nil {
			log.Fatalf("ERROR: Failed to load attributes: %v", err)
		}
		log.Printf("Loaded %d attributes", len(attrStore.GetAll()))
	}

	// Initialize AI generator
	generator := ai.NewGenerator(config.OllamaURL, config.ModelName, config.BannedTerms)

	// Ping Ollama
	log.Printf("Checking Ollama connection...")
	if err := generator.Ping(); err != nil {
		log.Fatalf("ERROR: Cannot connect to Ollama: %v", err)
	}
	log.Printf("✓ Ollama is available")

	// Register providers
	providerList := []providers.ContentProvider{
		asciidoc.NewProvider(),
		// Future: Add docbook.NewProvider() here
	}

	// Discover files
	scanner := discovery.NewScanner(config.RootDir, providerList)
	log.Printf("Scanning for files...")
	
	allFiles, err := scanner.Scan()
	if err != nil {
		log.Fatalf("ERROR: Failed to scan directory: %v", err)
	}
	
	log.Printf("Found %d potential files", len(allFiles))

	// Filter by type
	files := discovery.FilterByType(allFiles, config.FileType)
	log.Printf("Processing %d files (type filter: %s)", len(files), config.FileType)

	if len(files) == 0 {
		log.Printf("No files to process")
		return
	}

	// Handle remove-descriptions mode
	if config.RemoveDescriptions {
		log.Printf("*** REMOVE DESCRIPTIONS MODE ***")
		if config.DryRun {
			log.Printf("*** DRY RUN - No files will be modified ***")
		}
		
		removedCount := 0
		for _, path := range files {
			// Find appropriate provider
			var provider providers.ContentProvider
			for _, prov := range providerList {
				if prov.CanHandle(path) {
					provider = prov
					break
				}
			}
			
			if provider == nil {
				continue
			}
			
			// Check if file has descriptions to remove
			hasDesc, err := provider.HasExistingDescription(path)
			if err != nil {
				log.Printf("ERROR checking %s: %v", path, err)
				continue
			}
			
			if !hasDesc {
				continue
			}
			
			// Remove descriptions
			if err := provider.RemoveDescriptions(path, config.DryRun); err != nil {
				log.Printf("ERROR removing descriptions from %s: %v", path, err)
				continue
			}
			
			removedCount++
			if config.DryRun {
				log.Printf("[DRY RUN] Would remove descriptions from: %s", path)
			} else {
				log.Printf("REMOVED descriptions from: %s", path)
			}
		}
		
		log.Printf("\n=== Removal Complete ===")
		log.Printf("Files processed: %d", len(files))
		if config.DryRun {
			log.Printf("Would remove descriptions from: %d files", removedCount)
		} else {
			log.Printf("Removed descriptions from: %d files", removedCount)
		}
		return
	}

	if config.DryRun {
		log.Printf("*** DRY RUN MODE - No files will be modified ***")
	}

	// Process files
	proc := processor.NewProcessor(providerList, generator, attrStore, config)
	
	startTime := time.Now()
	results, err := proc.ProcessFiles(files)
	if err != nil {
		log.Fatalf("ERROR: Processing failed: %v", err)
	}
	duration := time.Since(startTime)

	// Summary statistics
	stats := calculateStats(results)
	
	log.Printf("\n=== Processing Complete ===")
	log.Printf("Total time: %.2f seconds", duration.Seconds())
	log.Printf("Files processed: %d", stats.Total)
	log.Printf("Added: %d", stats.Added)
	log.Printf("Replaced: %d", stats.Replaced)
	log.Printf("Skipped: %d", stats.Skipped)
	log.Printf("Errors: %d", stats.Errors)
	log.Printf("Warnings: %d", stats.Warnings)

	if config.DryRun {
		log.Printf("Dry run: %d files would be modified", stats.DryRun)
	}

	// TODO: Generate HTML report if requested
	if config.HTMLLogPath != "" {
		log.Printf("HTML report generation not yet implemented")
		log.Printf("Will be saved to: %s", config.HTMLLogPath)
	}
}

// parseFlags parses command-line flags
func parseFlags() *models.Config {
	config := &models.Config{
		BuildAttributes: make(map[string]string),
	}

	flag.StringVar(&config.RootDir, "root", "", "Root directory of documentation files (required)")
	flag.StringVar(&config.ModelName, "model", "llama3.1:8b", "Ollama model to use")
	flag.StringVar(&config.OllamaURL, "ollama-url", getEnv("OLLAMA_URL", "http://127.0.0.1:11434"), "Ollama API base URL")
	flag.StringVar(&config.AttributesFile, "attributes-file", "", "Path to attributes file")
	flag.StringVar(&config.FileType, "type", "all", "File type to process: asciidoc, docbook, all")
	flag.BoolVar(&config.ForceOverwrite, "force-overwrite", false, "Overwrite existing descriptions")
	flag.BoolVar(&config.DryRun, "dry-run", false, "Preview changes without writing files")
	flag.BoolVar(&config.RemoveDescriptions, "remove-descriptions", false, "Remove all description attributes instead of generating new ones")
	flag.StringVar(&config.HTMLLogPath, "html-log", "", "Path to HTML log output")
	flag.StringVar(&config.ReportTitle, "report-title", "Description Generation Report", "Custom HTML report title")
	
	var bannedTermsStr string
	flag.StringVar(&bannedTermsStr, "banned-terms", "", "Comma-separated list of terms to ban")

	// Build attributes for conditional processing
	var buildAttrsFlag arrayFlags
	flag.Var(&buildAttrsFlag, "a", "Set build attribute (e.g., -a build-type=product)")

	flag.Parse()

	// Validate required flags
	if config.RootDir == "" {
		fmt.Fprintf(os.Stderr, "ERROR: --root flag is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	// Parse banned terms
	if bannedTermsStr != "" {
		config.BannedTerms = strings.Split(bannedTermsStr, ",")
		for i := range config.BannedTerms {
			config.BannedTerms[i] = strings.TrimSpace(config.BannedTerms[i])
		}
	}

	// Parse build attributes
	for _, attr := range buildAttrsFlag {
		parts := strings.SplitN(attr, "=", 2)
		if len(parts) == 2 {
			config.BuildAttributes[parts[0]] = parts[1]
		}
	}

	return config
}

// arrayFlags allows multiple flag values
type arrayFlags []string

func (a *arrayFlags) String() string {
	return strings.Join(*a, ",")
}

func (a *arrayFlags) Set(value string) error {
	*a = append(*a, value)
	return nil
}

// getEnv gets environment variable with default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Stats holds processing statistics
type Stats struct {
	Total    int
	Added    int
	Replaced int
	Skipped  int
	Errors   int
	Warnings int
	DryRun   int
}

// calculateStats computes statistics from results
func calculateStats(results []*models.ProcessingResult) Stats {
	stats := Stats{Total: len(results)}

	for _, result := range results {
		switch result.Status {
		case models.StatusAdded:
			stats.Added++
		case models.StatusReplaced:
			stats.Replaced++
		case models.StatusSkipped:
			stats.Skipped++
		case models.StatusError:
			stats.Errors++
		case models.StatusWarning:
			stats.Warnings++
		case models.StatusDryRun:
			stats.DryRun++
		}
	}

	return stats
}
