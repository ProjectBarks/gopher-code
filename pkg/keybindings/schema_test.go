package keybindings

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestContextCount(t *testing.T) {
	// TS source defines 20 contexts (18 original + Scroll + MessageActions).
	if got := len(AllContexts); got != 20 {
		t.Fatalf("AllContexts: got %d, want 20", got)
	}
}

func TestActionCount(t *testing.T) {
	if got := len(AllActions); got != 104 {
		t.Fatalf("AllActions: got %d, want 104", got)
	}
}

func TestEveryContextHasDescription(t *testing.T) {
	for _, c := range AllContexts {
		desc, ok := ContextDescriptions[c]
		if !ok {
			t.Errorf("context %q missing from ContextDescriptions", c)
		} else if desc == "" {
			t.Errorf("context %q has empty description", c)
		}
	}
}

func TestDescriptionMapExactSize(t *testing.T) {
	if got, want := len(ContextDescriptions), len(AllContexts); got != want {
		t.Errorf("ContextDescriptions has %d entries, AllContexts has %d", got, want)
	}
}

func TestNoDuplicateContexts(t *testing.T) {
	seen := make(map[Context]bool)
	for _, c := range AllContexts {
		if seen[c] {
			t.Errorf("duplicate context: %q", c)
		}
		seen[c] = true
	}
}

func TestNoDuplicateActions(t *testing.T) {
	seen := make(map[Action]bool)
	for _, a := range AllActions {
		if seen[a] {
			t.Errorf("duplicate action: %q", a)
		}
		seen[a] = true
	}
}

func TestActionFormat(t *testing.T) {
	for _, a := range AllActions {
		s := string(a)
		if !strings.Contains(s, ":") {
			t.Errorf("action %q missing colon separator", a)
		}
		parts := strings.SplitN(s, ":", 2)
		if parts[0] == "" || parts[1] == "" {
			t.Errorf("action %q has empty namespace or name", a)
		}
	}
}

func TestValidContext(t *testing.T) {
	if !ValidContext(ContextGlobal) {
		t.Error("ValidContext(ContextGlobal) = false")
	}
	if ValidContext(Context("Nonexistent")) {
		t.Error("ValidContext(Nonexistent) = true")
	}
}

func TestValidAction(t *testing.T) {
	if !ValidAction(ActionAppInterrupt) {
		t.Error("ValidAction(ActionAppInterrupt) = false")
	}
	if ValidAction(Action("fake:action")) {
		t.Error("ValidAction(fake:action) = true")
	}
}

func TestKeybindingsFileJSON(t *testing.T) {
	f := KeybindingsFile{
		Schema: "https://example.com/schema.json",
		Docs:   "https://example.com/docs",
		Bindings: []KeybindingBlock{
			{
				Context:  ContextGlobal,
				Bindings: map[string]string{"ctrl+c": string(ActionAppInterrupt)},
			},
		},
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded KeybindingsFile
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Schema != f.Schema {
		t.Errorf("$schema: got %q, want %q", decoded.Schema, f.Schema)
	}
	if decoded.Docs != f.Docs {
		t.Errorf("$docs: got %q, want %q", decoded.Docs, f.Docs)
	}
	if len(decoded.Bindings) != 1 {
		t.Fatalf("bindings length: got %d, want 1", len(decoded.Bindings))
	}
	if decoded.Bindings[0].Context != ContextGlobal {
		t.Errorf("context: got %q, want %q", decoded.Bindings[0].Context, ContextGlobal)
	}
}

func TestExpectedContextValues(t *testing.T) {
	expected := []string{
		"Global", "Chat", "Autocomplete", "Confirmation", "Help",
		"Transcript", "HistorySearch", "Task", "ThemePicker", "Settings",
		"Tabs", "Attachments", "Footer", "MessageSelector", "MessageActions",
		"Scroll", "DiffDialog", "ModelPicker", "Select", "Plugin",
	}
	for i, want := range expected {
		if got := string(AllContexts[i]); got != want {
			t.Errorf("AllContexts[%d] = %q, want %q", i, got, want)
		}
	}
}

func TestExpectedDescriptions(t *testing.T) {
	cases := map[Context]string{
		ContextGlobal:          "Active everywhere, regardless of focus",
		ContextChat:            "When the chat input is focused",
		ContextHistorySearch:   "When searching command history (ctrl+r)",
		ContextMessageSelector: "When the message selector (rewind) is open",
		ContextPlugin:          "When the plugin dialog is open",
	}
	for ctx, want := range cases {
		if got := ContextDescriptions[ctx]; got != want {
			t.Errorf("ContextDescriptions[%q] = %q, want %q", ctx, got, want)
		}
	}
}
