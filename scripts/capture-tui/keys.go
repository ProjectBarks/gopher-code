package main

import "fmt"

// KeyToBytes converts a named key to its terminal escape sequence.
// Supports: Enter, Escape, Tab, Shift+Tab, Backspace, Delete,
// Up, Down, Left, Right, Home, End, PageUp, PageDown,
// Ctrl+A through Ctrl+Z, F1-F12.
func KeyToBytes(name string) ([]byte, error) {
	if b, ok := keyMap[name]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("unknown key name: %q (see keys.go for supported names)", name)
}

var keyMap = map[string][]byte{
	// Basic keys
	"Enter":     {'\r'},
	"Return":    {'\r'},
	"Escape":    {0x1b},
	"Esc":       {0x1b},
	"Tab":       {'\t'},
	"Shift+Tab": {0x1b, '[', 'Z'},
	"Backspace": {0x7f},
	"Delete":    {0x1b, '[', '3', '~'},
	"Space":     {' '},

	// Arrow keys
	"Up":    {0x1b, '[', 'A'},
	"Down":  {0x1b, '[', 'B'},
	"Right": {0x1b, '[', 'C'},
	"Left":  {0x1b, '[', 'D'},

	// Navigation
	"Home":     {0x1b, '[', 'H'},
	"End":      {0x1b, '[', 'F'},
	"PageUp":   {0x1b, '[', '5', '~'},
	"PageDown": {0x1b, '[', '6', '~'},
	"Insert":   {0x1b, '[', '2', '~'},

	// Ctrl+letter (A=1 through Z=26)
	"Ctrl+A": {0x01},
	"Ctrl+B": {0x02},
	"Ctrl+C": {0x03},
	"Ctrl+D": {0x04},
	"Ctrl+E": {0x05},
	"Ctrl+F": {0x06},
	"Ctrl+G": {0x07},
	"Ctrl+H": {0x08},
	"Ctrl+I": {0x09}, // same as Tab
	"Ctrl+J": {0x0a},
	"Ctrl+K": {0x0b},
	"Ctrl+L": {0x0c},
	"Ctrl+M": {0x0d}, // same as Enter
	"Ctrl+N": {0x0e},
	"Ctrl+O": {0x0f},
	"Ctrl+P": {0x10},
	"Ctrl+Q": {0x11},
	"Ctrl+R": {0x12},
	"Ctrl+S": {0x13},
	"Ctrl+T": {0x14},
	"Ctrl+U": {0x15},
	"Ctrl+V": {0x16},
	"Ctrl+W": {0x17},
	"Ctrl+X": {0x18},
	"Ctrl+Y": {0x19},
	"Ctrl+Z": {0x1a},

	// Function keys (xterm sequences)
	"F1":  {0x1b, 'O', 'P'},
	"F2":  {0x1b, 'O', 'Q'},
	"F3":  {0x1b, 'O', 'R'},
	"F4":  {0x1b, 'O', 'S'},
	"F5":  {0x1b, '[', '1', '5', '~'},
	"F6":  {0x1b, '[', '1', '7', '~'},
	"F7":  {0x1b, '[', '1', '8', '~'},
	"F8":  {0x1b, '[', '1', '9', '~'},
	"F9":  {0x1b, '[', '2', '0', '~'},
	"F10": {0x1b, '[', '2', '1', '~'},
	"F11": {0x1b, '[', '2', '3', '~'},
	"F12": {0x1b, '[', '2', '4', '~'},

	// Shift+Enter (xterm modifyOtherKeys)
	"Shift+Enter": {0x1b, '[', '2', '7', ';', '2', ';', '1', '3', '~'},
}
