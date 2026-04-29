package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/scribe/doc-meta-gen/internal/ai"
	"github.com/scribe/doc-meta-gen/internal/discovery"
	"github.com/scribe/doc-meta-gen/internal/models"
	"github.com/scribe/doc-meta-gen/internal/processor"
	"github.com/scribe/doc-meta-gen/internal/providers"
	"github.com/scribe/doc-meta-gen/internal/providers/asciidoc"
	"github.com/scribe/doc-meta-gen/internal/providers/docbook"
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

	// Load attributes / entities
	attrStore := attributes.NewStore()
	if len(config.AttributesFiles) > 0 {
		log.Printf("Loading attributes from: %s", strings.Join(config.AttributesFiles, ", "))
		if err := attrStore.LoadFromFiles(config.AttributesFiles, config.BuildAttributes); err != nil {
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
		docbook.NewProvider(),
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

	// Generate HTML report if requested
	if config.HTMLLogPath != "" {
		if err := generateHTMLReport(config.HTMLLogPath, config, stats, results, duration); err != nil {
			log.Printf("WARNING: Failed to write HTML report: %v", err)
		} else {
			log.Printf("HTML report saved to: %s", config.HTMLLogPath)
		}
	}
}

func generateHTMLReport(
	outputPath string,
	config *models.Config,
	stats Stats,
	results []*models.ProcessingResult,
	duration time.Duration,
) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create report file: %w", err)
	}
	defer f.Close()

	now := time.Now().Format("2006-01-02 15:04:05")

	// Helper: shorten path to start from the doc-* directory
	truncatePath := func(p string) string {
		parts := strings.Split(filepath.ToSlash(p), "/")
		for i, part := range parts {
			if strings.HasPrefix(part, "doc-") {
				return strings.Join(parts[i:], "/")
			}
		}
		if len(parts) >= 2 {
			return strings.Join(parts[len(parts)-2:], "/")
		}
		return p
	}

	statusColour := func(s models.Status) string {
		switch s {
		case models.StatusAdded, models.StatusReplaced:
			return "bg-green-500 border-green-500"
		case models.StatusUpdated:
			return "bg-indigo-500 border-indigo-500"
		case models.StatusDryRun:
			return "bg-blue-500 border-blue-500"
		case models.StatusSkipped:
			return "bg-yellow-500 border-yellow-500"
		case models.StatusWarning:
			return "bg-yellow-500 border-yellow-500"
		case models.StatusError:
			return "bg-red-500 border-red-500"
		default:
			return "bg-gray-500 border-gray-500"
		}
	}

	w := bufio.NewWriter(f)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en" class="bg-slate-100">
<head>
  <meta charset="UTF-8">
  <title>%s: AI Generated Meta Descriptions</title>
  <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="font-sans text-slate-800">
<div class="container mx-auto p-4 sm:p-6 lg:p-8">

  <div class="mb-8">
    <h1 class="text-4xl font-bold text-slate-900">%s</h1>
    <p class="text-xl text-slate-600 mt-1">AI Generated Meta Descriptions</p>
    <p class="text-sm text-slate-400 mt-2">Report generated on %s</p>
  </div>

  <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
    <div class="bg-white p-5 rounded-xl shadow-lg flex items-center justify-between">
      <div><div class="text-sm font-medium text-slate-500">Files Processed</div><div class="mt-1 text-3xl font-semibold text-slate-900">%d</div></div>
    </div>
    <div class="bg-white p-5 rounded-xl shadow-lg flex items-center justify-between">
      <div><div class="text-sm font-medium text-slate-500">Files Changed</div><div class="mt-1 text-3xl font-semibold text-green-600">%d</div></div>
    </div>
    <div class="bg-white p-5 rounded-xl shadow-lg flex items-center justify-between">
      <div><div class="text-sm font-medium text-slate-500">Processing Time</div><div class="mt-1 text-3xl font-semibold text-slate-900">%.2fs</div></div>
    </div>
    <div class="bg-white p-5 rounded-xl shadow-lg flex items-center justify-between">
      <div><div class="text-sm font-medium text-slate-500">Model Used</div><div class="mt-1 text-2xl font-semibold text-slate-900 truncate">%s</div></div>
    </div>
  </div>

  <div class="bg-white rounded-xl shadow-lg overflow-hidden">
    <div class="p-5 border-b border-slate-200 flex flex-col sm:flex-row justify-between items-center">
      <div>
        <h2 class="text-xl font-semibold text-slate-900">Processing Log</h2>
        <p class="text-sm text-slate-500 mt-1">Detailed log of all file operations.</p>
      </div>
      <input type="text" id="searchInput" onkeyup="filterTable()" placeholder="Filter by path, status, or details..."
        class="mt-4 sm:mt-0 w-full sm:w-64 px-3 py-2 border border-slate-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
    </div>
    <div class="overflow-x-auto">
      <table class="min-w-full" id="logTable">
        <thead class="bg-slate-100 text-slate-600 text-sm font-semibold uppercase">
          <tr>
            <th class="py-3 px-6 text-left">File</th>
            <th class="py-3 px-6 text-left">Type</th>
            <th class="py-3 px-6 text-left">Status</th>
            <th class="py-3 px-6 text-left">Chars</th>
            <th class="py-3 px-6 text-left">Description / Details</th>
          </tr>
        </thead>
        <tbody class="text-sm">
`, htmlEscape(config.ReportTitle), htmlEscape(config.ReportTitle), now,
		stats.Total, stats.Added+stats.Replaced+stats.DryRun,
		duration.Seconds(), htmlEscape(config.ModelName))

	for _, r := range results {
		col := statusColour(r.Status)
		bgCol := strings.Split(col, " ")[0]
		borderCol := strings.Split(col, " ")[1]
		detail := r.Description
		if detail == "" {
			detail = r.Details
		}
		chars := ""
		if r.CharCount > 0 {
			chars = fmt.Sprintf("%d", r.CharCount)
		}
		fmt.Fprintf(w, `          <tr class="border-b border-slate-200 hover:bg-slate-50">
            <td class="py-3 px-6 font-medium text-slate-700 border-l-4 %s" title="%s">%s</td>
            <td class="py-3 px-6 whitespace-nowrap">%s</td>
            <td class="py-3 px-6"><span class="text-white text-xs font-bold px-2.5 py-1 rounded-full %s">%s</span></td>
            <td class="py-3 px-6 text-slate-500">%s</td>
            <td class="py-3 px-6 font-mono text-xs text-slate-600 break-words">%s</td>
          </tr>
`,
			borderCol,
			htmlEscape(r.FilePath),
			htmlEscape(truncatePath(r.FilePath)),
			htmlEscape(r.FileType),
			bgCol,
			htmlEscape(string(r.Status)),
			chars,
			htmlEscape(detail),
		)
	}

	fmt.Fprintf(w, `        </tbody>
      </table>
    </div>
  </div>

  <div class="mt-8 text-center text-sm text-slate-500">
    <p>Root: %s &bull; Type filter: %s</p>
  </div>
</div>
<script>
function filterTable() {
  var input = document.getElementById("searchInput").value.toUpperCase();
  var rows = document.getElementById("logTable").getElementsByTagName("tr");
  for (var i = 1; i < rows.length; i++) {
    var cells = rows[i].getElementsByTagName("td");
    var match = false;
    for (var j = 0; j < cells.length; j++) {
      if ((cells[j].textContent || cells[j].innerText).toUpperCase().indexOf(input) > -1) {
        match = true; break;
      }
    }
    rows[i].style.display = match ? "" : "none";
  }
}
</script>
</body>
</html>
`, htmlEscape(config.RootDir), htmlEscape(config.FileType))

	return w.Flush()
}

// htmlEscape escapes special HTML characters in a string.
func htmlEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&#34;",
	).Replace(s)
}

// parseFlags parses command-line flags
func parseFlags() *models.Config {
	config := &models.Config{
		BuildAttributes: make(map[string]string),
	}

	flag.StringVar(&config.RootDir, "root", "", "Root directory of documentation files (required)")
	flag.StringVar(&config.ModelName, "model", "qwen3:14b", "Ollama model to use")
	flag.StringVar(&config.OllamaURL, "ollama-url", getEnv("OLLAMA_URL", "http://127.0.0.1:11434"), "Ollama API base URL")
	var attrFilesFlag arrayFlags
	flag.Var(&attrFilesFlag, "attributes-file", "Path to an attributes or entities file (.adoc or .ent). Can be repeated.")
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

	config.AttributesFiles = []string(attrFilesFlag)

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
