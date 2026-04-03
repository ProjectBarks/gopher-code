# Batch 3 Notes — File Tools

## What was done

### FileReadTool: Offset semantics bug fix (CRITICAL)
The Go offset was "skip N lines" (`lineNum <= in.Offset`), but TS offset means "start reading FROM line N" (1-indexed). With offset=2, Go was skipping lines 1 AND 2 (reading from line 3), while TS reads from line 2. Fixed by changing `<=` to `<` with a guard for offset > 0.

### FileReadTool: Blocked device paths
TS blocks dangerous device files (/dev/zero, /dev/random, /dev/stdin, /dev/tty, etc.) that would hang the process. Go now blocks the same 11 paths.

### FileReadTool: Tilde expansion
TS uses `expandPath()` to handle `~/` paths. Go now expands `~/` to the home directory.

### FileReadTool: Empty file and offset warnings
TS returns `<system-reminder>` warnings when: (a) file is empty, (b) offset is beyond EOF. Go was silently returning empty string. Now matches TS warning messages.

### FileEditTool: MaxEditFileSize 10MB → 1GiB
TS uses 1 GiB (V8/Bun string limit). Go had 10 MB which is 100x too restrictive. Fixed.

### FileEditTool: Quote normalization
TS has `findActualString()` and `normalizeQuotes()` — when the model sends straight quotes but the file has curly quotes (common in prose/docs), the TS finds the match via normalization. Added this to Go with proper rune-level indexing (curly quotes are 3 bytes vs 1 byte for straight quotes).

### FileEditTool: Deletion trailing newline handling
TS `applyEditToFile()` has special logic: when deleting text (new_string=""), if the old_string doesn't end with `\n` but appears followed by `\n` in the file, it also strips the trailing newline. This prevents leaving blank lines after deletions.

### GrepTool: Missing .bzr VCS exclusion
TS excludes 6 VCS dirs: .git, .svn, .hg, .bzr, .jj, .sl. Go had only 5 (missing .bzr). Fixed.

## What's NOT done (deferred)

### FileReadTool: Image/PDF/Notebook support
TS has full image processing (resize, compress, token budgeting), PDF page extraction, and Jupyter notebook reading. Go handles text only. Would need image/PDF libraries.

### FileReadTool: File deduplication/staleness
TS caches last read + mtime and returns "file_unchanged" stub if the file hasn't changed. Go doesn't deduplicate reads.

### FileWriteTool: Line ending normalization
TS enforces LF line endings. Go doesn't normalize. Minor concern for cross-platform.

### GlobTool: Sorting strategy
TS sorts by mtime (most recently modified first). Go sorts alphabetically. Different behavior but both are valid approaches — mtime is more useful for large repos. Could be changed in a future pass.

### tools/shared: spawnMultiAgent
TS has a complex multi-agent spawning system with tmux pane management. This is covered by Batch 5 (Agent & Team Tools).

### tools/shared: gitOperationTracking
TS tracks git operations (commits, PR links, branch actions) for attribution and analytics. Covered by Batch 16 (Git utils).

## Patterns noticed

1. **Rune vs byte indexing** is a recurring concern. Go's `strings.Index` returns byte offsets but JS `indexOf` returns character offsets. Any normalization that changes byte widths (curly→straight quotes, UTF-8 multibyte) needs rune-level conversion. Watch for this in other tools that do string matching.

2. **System-reminder format** `<system-reminder>...</system-reminder>` is used by TS for in-band warnings to the model. Go should use this consistently for similar warnings (empty files, offset-beyond-EOF, staleness warnings).

3. **File permission handling** — TS does not appear to use atomic writes (temp file + rename) for FileWrite or FileEdit. Both write directly via `writeTextContent()`. Go's approach of direct `os.WriteFile()` matches.
