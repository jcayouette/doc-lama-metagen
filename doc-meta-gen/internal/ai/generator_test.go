package ai

import (
	"strings"
	"testing"
)

type stubModel struct {
	responses []string
	prompts   []string
}

func (s *stubModel) Generate(model, prompt string) (string, error) {
	s.prompts = append(s.prompts, prompt)
	if len(s.responses) == 0 {
		return "", errNoResponse
	}
	response := s.responses[0]
	s.responses = s.responses[1:]
	return response, nil
}

func (s *stubModel) Ping() error { return nil }

type staticError string

func (e staticError) Error() string { return string(e) }

const errNoResponse = staticError("no response")

func TestSanitizeDoesNotSliceMidSentence(t *testing.T) {
	g := NewGenerator("http://127.0.0.1:9", "test", nil)
	long := strings.TrimSpace(strings.Repeat("cluster ", 30)) + " during different phases of rollout"
	got := g.sanitize(long)
	if len(got) <= 160 {
		t.Fatalf("long sentence was sliced to %d characters: %s", len(got), got)
	}
	if strings.HasSuffix(strings.ToLower(got), "different") && len(got) <= 160 {
		t.Fatalf("saved a mid-sentence slice: %s", got)
	}
}

func TestSanitizeKeepsFinishedSentenceAndDropsPeriod(t *testing.T) {
	g := NewGenerator("http://127.0.0.1:9", "test", nil)
	sentence := "See the personas, such as platform engineers and application developers, and the tasks each role performs across clusters"
	got := g.sanitize(sentence + ". Extra commentary follows.")
	if got != sentence {
		t.Fatalf("got %q (%d)", got, len(got))
	}
}

func TestRejectFleetFragments(t *testing.T) {
	g := NewGenerator("http://127.0.0.1:9", "test", nil)
	cases := []struct {
		title string
		desc  string
	}{
		{
			"Key Personas and Their Roles",
			"Identify the main people involved in a project and describe their responsibilities to help team members understand who is responsible for what during different",
		},
		{
			"Other versions",
			"Find all previous versions of Fleet, along with information about what changed in each release, by visiting the official GitHub Release page where every update",
		},
		{
			"Downstream resource propagation",
			"Automatically copying resources to downstream clusters simplifies managing dependencies for charts and ensures workloads use the latest versions of referenced",
		},
		{
			"fleet",
			"Manage Fleet operations by using commands to create, deploy, clean up, monitor, and analyze cluster setups, helping you control",
		},
		{
			"fleet bundlediff",
			"Display bundle diffs from resource status to see changes between the desired state from Git or Helm and the actual state in the cluster by extracting",
		},
		{
			"Experimental downstream resource",
			"Copy resources such as secrets and config maps from an upstream cluster to targeted downstream clusters automatically before deploying workloads to ensure",
		},
	}
	for _, tc := range cases {
		if why := g.reject(tc.title, tc.desc); why == "" {
			t.Errorf("accepted a fragment for %s: %s", tc.title, tc.desc)
		}
	}
}

func TestAcceptSentenceThatUsesTheTitle(t *testing.T) {
	g := NewGenerator("http://127.0.0.1:9", "test", nil)
	desc := "See the personas, such as platform engineers and application developers, and the tasks each role performs across clusters"
	if why := g.reject("Key Personas and Their Roles", desc); why != "" {
		t.Fatal(why)
	}
	if why := g.reject("Overview", "Show how the controller copies bundles from git to each downstream cluster and reports status back to the management cluster"); why != "" {
		t.Fatal(why)
	}
	configured := "Configure Fleet by using Helm values in a ConfigMap named rancher-config in the cattle-system namespace on the management cluster"
	if why := g.reject("Configuration", configured); why != "" {
		t.Fatal(why)
	}
}

func TestGenerateDescriptionRetriesAFragment(t *testing.T) {
	stub := &stubModel{responses: []string{
		"Identify the main people involved in a project and describe their responsibilities to help team members understand who is responsible for what during different",
		"See the personas, such as platform engineers and application developers, and the tasks each role performs across clusters.",
	}}
	g := &Generator{client: stub, model: "test"}
	got, err := g.GenerateDescription("Platform engineers and application developers are the personas on this page.", "Key Personas and Their Roles")
	if err != nil {
		t.Fatal(err)
	}
	want := "See the personas, such as platform engineers and application developers, and the tasks each role performs across clusters"
	if got != want {
		t.Fatalf("got %q", got)
	}
	if len(stub.prompts) != 2 {
		t.Fatalf("attempts = %d", len(stub.prompts))
	}
	if strings.Contains(stub.prompts[1], "120-155") || strings.Contains(stub.prompts[1], "stay at or under") {
		t.Fatal("follow-up prompt still sets a character budget")
	}
	if !strings.Contains(stub.prompts[1], "during different") {
		t.Fatal("follow-up prompt did not show the rejected sentence")
	}
}

func TestGenerateDescriptionRefusesThreeFragments(t *testing.T) {
	fragment := "Identify the main people involved in a project and describe their responsibilities to help team members understand who is responsible for what during different"
	stub := &stubModel{responses: []string{fragment, fragment, fragment}}
	g := &Generator{client: stub, model: "test"}
	got, err := g.GenerateDescription("Platform engineers maintain the clusters.", "Key Personas and Their Roles")
	if err == nil {
		t.Fatalf("saved fragment: %s", got)
	}
	if strings.Contains(got, "during different") {
		t.Fatal("returned the fragment as a description")
	}
}

func TestKeepIfCompleteDropsACutOffRewrite(t *testing.T) {
	g := NewGenerator("http://127.0.0.1:9", "test", nil)
	current := "See the personas, such as platform engineers and application developers, and the tasks each role performs across clusters"
	rewrite := "Identify the main people involved in a project and describe their responsibilities to help team members understand who is responsible for what during different."
	got := g.KeepIfComplete("Key Personas and Their Roles", current, rewrite)
	if got != current {
		t.Fatalf("grammar pass replaced a good sentence with %q", got)
	}
}

func TestPromptDoesNotAskForACharacterBudget(t *testing.T) {
	g := NewGenerator("http://127.0.0.1:9", "test", nil)
	prompt := g.buildPrompt("content", "Key Personas and Their Roles", "")
	if strings.Contains(prompt, "120-160") {
		t.Fatal("primary prompt still sets a character budget")
	}
	if !strings.Contains(prompt, "End the sentence with a period") {
		t.Fatal("primary prompt no longer asks for a finished sentence")
	}
}
