package keybindings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKeystroke(t *testing.T) {
	tests := []struct {
		input string
		want  ParsedKeystroke
	}{
		{"ctrl+c", ParsedKeystroke{Key: "c", Ctrl: true}},
		{"control+x", ParsedKeystroke{Key: "x", Ctrl: true}},
		{"alt+f", ParsedKeystroke{Key: "f", Alt: true}},
		{"opt+f", ParsedKeystroke{Key: "f", Alt: true}},
		{"option+f", ParsedKeystroke{Key: "f", Alt: true}},
		{"meta+f", ParsedKeystroke{Key: "f", Meta: true}},
		{"cmd+c", ParsedKeystroke{Key: "c", Super: true}},
		{"command+c", ParsedKeystroke{Key: "c", Super: true}},
		{"super+c", ParsedKeystroke{Key: "c", Super: true}},
		{"win+c", ParsedKeystroke{Key: "c", Super: true}},
		{"shift+k", ParsedKeystroke{Key: "k", Shift: true}},
		{"ctrl+shift+k", ParsedKeystroke{Key: "k", Ctrl: true, Shift: true}},
		{"esc", ParsedKeystroke{Key: "escape"}},
		{"return", ParsedKeystroke{Key: "enter"}},
		{"space", ParsedKeystroke{Key: " "}},
		{"\u2191", ParsedKeystroke{Key: "up"}},
		{"\u2193", ParsedKeystroke{Key: "down"}},
		{"\u2190", ParsedKeystroke{Key: "left"}},
		{"\u2192", ParsedKeystroke{Key: "right"}},
		{"a", ParsedKeystroke{Key: "a"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ParseKeystroke(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseChord(t *testing.T) {
	t.Run("single keystroke", func(t *testing.T) {
		chord := ParseChord("ctrl+c")
		require.Len(t, chord, 1)
		assert.True(t, chord[0].Ctrl)
		assert.Equal(t, "c", chord[0].Key)
	})

	t.Run("two-step chord", func(t *testing.T) {
		chord := ParseChord("ctrl+x ctrl+e")
		require.Len(t, chord, 2)
		assert.True(t, chord[0].Ctrl)
		assert.Equal(t, "x", chord[0].Key)
		assert.True(t, chord[1].Ctrl)
		assert.Equal(t, "e", chord[1].Key)
	})

	t.Run("lone space is space key", func(t *testing.T) {
		chord := ParseChord(" ")
		require.Len(t, chord, 1)
		assert.Equal(t, " ", chord[0].Key)
	})
}

func TestKeystrokeToString(t *testing.T) {
	tests := []struct {
		name string
		ks   ParsedKeystroke
		want string
	}{
		{"plain key", ParsedKeystroke{Key: "k"}, "k"},
		{"ctrl+c", ParsedKeystroke{Key: "c", Ctrl: true}, "ctrl+c"},
		{"all modifiers", ParsedKeystroke{Key: "x", Ctrl: true, Alt: true, Shift: true, Meta: true, Super: true}, "ctrl+alt+shift+meta+cmd+x"},
		{"escape display", ParsedKeystroke{Key: "escape"}, "Esc"},
		{"arrow up display", ParsedKeystroke{Key: "up"}, "\u2191"},
		{"space display", ParsedKeystroke{Key: " "}, "Space"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, KeystrokeToString(tt.ks))
		})
	}
}

func TestChordToString(t *testing.T) {
	chord := Chord{
		{Key: "k", Ctrl: true},
		{Key: "s", Ctrl: true},
	}
	assert.Equal(t, "ctrl+k ctrl+s", ChordToString(chord))
}

func TestKeystrokeToDisplayString(t *testing.T) {
	ks := ParsedKeystroke{Key: "c", Alt: true, Super: true}

	t.Run("macos uses opt and cmd", func(t *testing.T) {
		assert.Equal(t, "opt+cmd+c", KeystrokeToDisplayString(ks, PlatformMacOS))
	})
	t.Run("linux uses alt and super", func(t *testing.T) {
		assert.Equal(t, "alt+super+c", KeystrokeToDisplayString(ks, PlatformLinux))
	})

	t.Run("meta collapses into alt display", func(t *testing.T) {
		metaKs := ParsedKeystroke{Key: "f", Meta: true}
		assert.Equal(t, "opt+f", KeystrokeToDisplayString(metaKs, PlatformMacOS))
		assert.Equal(t, "alt+f", KeystrokeToDisplayString(metaKs, PlatformLinux))
	})
}

func TestParseBindings(t *testing.T) {
	blocks := []KeybindingBlock{
		{
			Context: ContextChat,
			Bindings: map[string]string{
				"ctrl+s": "save",
				"ctrl+z": "undo",
			},
		},
		{
			Context: ContextGlobal,
			Bindings: map[string]string{
				"ctrl+q": "quit",
			},
		},
	}
	bindings := ParseBindings(blocks)
	require.Len(t, bindings, 3)

	// Collect actions to verify all three are present (map iteration order is random).
	actions := map[string]bool{}
	for _, b := range bindings {
		actions[b.Action] = true
		require.Len(t, b.Chord, 1)
	}
	assert.True(t, actions["save"])
	assert.True(t, actions["undo"])
	assert.True(t, actions["quit"])
}
