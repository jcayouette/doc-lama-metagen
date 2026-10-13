package asciidoc

import (
	"strings"
	"testing"
)

func TestExtractPlainTextKeepsTableCellsAndSkipsListings(t *testing.T) {
	p := NewProvider()
	text := `
[cols="1,2,2", options="header"]
|===
|Persona |Responsibilities

|Platform Engineers
a|* Set up and manage the installation across clusters.
|Application Developers
a|* Structure application Helm charts in Git.

|Security Administrators
a|* Define and enforce security policies as code.
|===

----
this listing must not reach the model
----
`
	got := p.extractPlainText(text)
	for _, want := range []string{"Platform Engineers", "Application Developers", "Security Administrators", "security policies"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "must not reach") {
		t.Fatalf("listing block was included: %s", got)
	}
}
