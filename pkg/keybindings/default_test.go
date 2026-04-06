package keybindings

import (
	"runtime"
	"strings"
	"testing"
)

// helper: index blocks by context for easy lookup.
func blocksByContext(t *testing.T) map[Context]map[string]string {
	t.Helper()
	m := make(map[Context]map[string]string)
	for _, b := range DefaultBindings() {
		m[b.Context] = b.Bindings
	}
	return m
}

func TestDefaultBindingsContextCount(t *testing.T) {
	blocks := DefaultBindings()
	if got := len(blocks); got != 20 {
		t.Errorf("context count = %d, want 20", got)
	}
}

func TestDefaultBindingsTotalCount(t *testing.T) {
	blocks := DefaultBindings()
	total := 0
	for _, b := range blocks {
		total += len(b.Bindings)
	}
	if total < 130 {
		t.Errorf("total binding count = %d, want >= 130", total)
	}
}

func TestAllContextsPresent(t *testing.T) {
	bc := blocksByContext(t)
	expected := []Context{
		ContextGlobal, ContextChat, ContextAutocomplete, ContextSettings,
		ContextConfirmation, ContextTabs, ContextTranscript, ContextHistorySearch,
		ContextTask, ContextThemePicker, ContextScroll, ContextHelp,
		ContextAttachments, ContextFooter, ContextMessageSelector,
		ContextMessageActions, ContextDiffDialog, ContextModelPicker,
		ContextSelect, ContextPlugin,
	}
	for _, ctx := range expected {
		if _, ok := bc[ctx]; !ok {
			t.Errorf("missing context %q", ctx)
		}
	}
}

func TestGlobalCtrlC(t *testing.T) {
	bc := blocksByContext(t)
	action, ok := bc[ContextGlobal]["ctrl+c"]
	if !ok {
		t.Fatal("Global context missing ctrl+c binding")
	}
	if action != string(ActionAppInterrupt) {
		t.Errorf("ctrl+c action = %q, want %q", action, ActionAppInterrupt)
	}
}

func TestGlobalCtrlD(t *testing.T) {
	bc := blocksByContext(t)
	action, ok := bc[ContextGlobal]["ctrl+d"]
	if !ok {
		t.Fatal("Global context missing ctrl+d binding")
	}
	if action != string(ActionAppExit) {
		t.Errorf("ctrl+d action = %q, want %q", action, ActionAppExit)
	}
}

func TestChatChordCtrlXCtrlK(t *testing.T) {
	bc := blocksByContext(t)
	action, ok := bc[ContextChat]["ctrl+x ctrl+k"]
	if !ok {
		t.Fatal("Chat context missing ctrl+x ctrl+k chord binding")
	}
	if action != string(ActionChatKillAgents) {
		t.Errorf("ctrl+x ctrl+k action = %q, want %q", action, ActionChatKillAgents)
	}
}

func TestChatChordCtrlXCtrlE(t *testing.T) {
	bc := blocksByContext(t)
	action, ok := bc[ContextChat]["ctrl+x ctrl+e"]
	if !ok {
		t.Fatal("Chat context missing ctrl+x ctrl+e chord binding")
	}
	if action != string(ActionChatExternalEditor) {
		t.Errorf("ctrl+x ctrl+e action = %q, want %q", action, ActionChatExternalEditor)
	}
}

func TestChatModeCycleKeyPlatform(t *testing.T) {
	bc := blocksByContext(t)
	var wantKey string
	if runtime.GOOS == "windows" {
		wantKey = "meta+m"
	} else {
		wantKey = "shift+tab"
	}
	action, ok := bc[ContextChat][wantKey]
	if !ok {
		t.Fatalf("Chat context missing %q binding", wantKey)
	}
	if action != string(ActionChatCycleMode) {
		t.Errorf("%s action = %q, want %q", wantKey, action, ActionChatCycleMode)
	}
}

func TestChatImagePasteKeyPlatform(t *testing.T) {
	bc := blocksByContext(t)
	var wantKey string
	if runtime.GOOS == "windows" {
		wantKey = "alt+v"
	} else {
		wantKey = "ctrl+v"
	}
	action, ok := bc[ContextChat][wantKey]
	if !ok {
		t.Fatalf("Chat context missing %q binding", wantKey)
	}
	if action != string(ActionChatImagePaste) {
		t.Errorf("%s action = %q, want %q", wantKey, action, ActionChatImagePaste)
	}
}

func TestConfirmationShiftTab(t *testing.T) {
	bc := blocksByContext(t)
	action, ok := bc[ContextConfirmation]["shift+tab"]
	if !ok {
		t.Fatal("Confirmation context missing shift+tab binding")
	}
	if action != string(ActionConfirmCycleMode) {
		t.Errorf("shift+tab action = %q, want %q", action, ActionConfirmCycleMode)
	}
}

func TestScrollCopyBindings(t *testing.T) {
	bc := blocksByContext(t)
	for _, key := range []string{"ctrl+shift+c", "cmd+c"} {
		action, ok := bc[ContextScroll][key]
		if !ok {
			t.Errorf("Scroll context missing %q binding", key)
			continue
		}
		if action != string(ActionSelectionCopy) {
			t.Errorf("%s action = %q, want %q", key, action, ActionSelectionCopy)
		}
	}
}

func TestMessageActionsKittySuper(t *testing.T) {
	bc := blocksByContext(t)
	for _, key := range []string{"super+up", "super+down"} {
		if _, ok := bc[ContextMessageActions][key]; !ok {
			t.Errorf("MessageActions missing kitty %q variant", key)
		}
	}
}

func TestChordsContainSpace(t *testing.T) {
	blocks := DefaultBindings()
	chords := 0
	for _, b := range blocks {
		for key := range b.Bindings {
			if strings.Contains(key, " ") {
				chords++
			}
		}
	}
	if chords < 2 {
		t.Errorf("found %d chord bindings, want >= 2", chords)
	}
}

func TestPerContextBindingCounts(t *testing.T) {
	bc := blocksByContext(t)
	checks := map[Context]int{
		ContextAutocomplete:  4,
		ContextTask:          1,
		ContextHelp:          1,
		ContextPlugin:        2,
		ContextTabs:          4,
		ContextHistorySearch: 5,
		ContextDiffDialog:    6,
	}
	for ctx, want := range checks {
		if got := len(bc[ctx]); got != want {
			t.Errorf("context %q has %d bindings, want %d", ctx, got, want)
		}
	}
}
