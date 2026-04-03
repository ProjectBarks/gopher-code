module github.com/projectbarks/gopher-code

go 1.25.8

require (
	charm.land/bubbles/v2 v2.1.0
	// Terminal UI — Charm v2 stack (March 2026)
	charm.land/bubbletea/v2 v2.0.2
	charm.land/glamour/v2 v2.0.0
	charm.land/huh/v2 v2.0.3
	charm.land/lipgloss/v2 v2.0.2
	charm.land/log/v2 v2.0.0
	github.com/alecthomas/chroma/v2 v2.23.1
	github.com/bmatcuk/doublestar/v4 v4.10.0
	github.com/charmbracelet/x/ansi v0.11.6
	github.com/charmbracelet/x/term v0.2.2
	github.com/coder/websocket v1.8.14

	// Image & PDF
	github.com/disintegration/imaging v1.6.2
	github.com/fsnotify/fsnotify v1.9.0

	// Git & GitHub
	github.com/go-git/go-git/v5 v5.17.2
	github.com/goccy/go-yaml v1.19.2
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/go-github/v84 v84.0.0
	github.com/google/renameio v1.0.1

	// Core
	github.com/google/uuid v1.6.0
	github.com/growthbook/growthbook-golang v0.2.8

	// API & streaming
	github.com/hashicorp/go-retryablehttp v0.7.8

	// Caching
	github.com/hashicorp/golang-lru/v2 v2.0.7

	// Configuration & validation
	github.com/knadh/koanf/v2 v2.3.4
	github.com/ledongthuc/pdf v0.0.0-20250511090121-5959a4027728

	// MCP (Model Context Protocol)
	github.com/mark3labs/mcp-go v0.46.0
	github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06
	github.com/saintfish/chardet v0.0.0-20230101081208-5e3ef4b5456d
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2
	github.com/sergi/go-diff v1.4.0
	github.com/spf13/cobra v1.10.2
	github.com/tmaxmax/go-sse v0.11.0

	// Security
	github.com/zalando/go-keyring v0.2.8

	// Observability
	go.opentelemetry.io/otel v1.42.0
	go.opentelemetry.io/otel/trace v1.42.0
	golang.org/x/oauth2 v0.36.0

	// Concurrency
	golang.org/x/sync v0.20.0

	// Shell parsing & file operations
	mvdan.cc/sh/v3 v3.13.0
)

require (
	github.com/creack/pty v1.1.24
	golang.org/x/net v0.47.0
	golang.org/x/text v0.31.0
)

require (
	dario.cat/mergo v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.1.6 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/catppuccin/go v0.2.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/charmbracelet/colorprofile v0.4.2 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20260205113103-524a6607adb8 // indirect
	github.com/charmbracelet/x/exp/ordered v0.1.0 // indirect
	github.com/charmbracelet/x/exp/slice v0.0.0-20250327172914-2fdc97757edf // indirect
	github.com/charmbracelet/x/exp/strings v0.0.0-20240722160745-212f7b056ed0 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
	github.com/danieljoos/wincred v1.2.3 // indirect
	github.com/dlclark/regexp2 v1.11.5 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.8.0 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/jsonschema-go v0.4.2 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/lucasb-eyer/go-colorful v1.3.0 // indirect
	github.com/mattn/go-runewidth v0.0.21 // indirect
	github.com/microcosm-cc/bluemonday v1.0.27 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/pjbgf/sha1cd v0.3.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/skeema/knownhosts v1.3.1 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	github.com/yuin/goldmark v1.7.8 // indirect
	github.com/yuin/goldmark-emoji v1.0.5 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.42.0 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56 // indirect
	golang.org/x/image v0.0.0-20191009234506-e7c1f5e7dbb8 // indirect
	golang.org/x/sys v0.42.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)
