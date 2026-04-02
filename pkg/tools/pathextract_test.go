package tools

import "testing"

// Source: tools/BashTool/pathValidation.ts

func TestFilterOutFlags(t *testing.T) {
	// Source: pathValidation.ts:126-139

	t.Run("removes_flags", func(t *testing.T) {
		result := FilterOutFlags([]string{"-la", "file.txt", "--verbose", "dir/"})
		if len(result) != 2 || result[0] != "file.txt" || result[1] != "dir/" {
			t.Errorf("expected [file.txt, dir/], got %v", result)
		}
	})

	t.Run("handles_double_dash", func(t *testing.T) {
		// Source: pathValidation.ts:131-133
		result := FilterOutFlags([]string{"--", "-dangerous-filename"})
		if len(result) != 1 || result[0] != "-dangerous-filename" {
			t.Errorf("expected [-dangerous-filename], got %v", result)
		}
	})

	t.Run("empty_args", func(t *testing.T) {
		result := FilterOutFlags(nil)
		if len(result) != 0 {
			t.Errorf("expected empty, got %v", result)
		}
	})
}

func TestExtractPaths(t *testing.T) {
	// Source: pathValidation.ts:190-310

	t.Run("cd_no_args", func(t *testing.T) {
		// Source: pathValidation.ts:195
		paths := ExtractPaths("cd", nil)
		if len(paths) != 1 {
			t.Fatalf("expected 1 path, got %d", len(paths))
		}
		// Should return home directory
	})

	t.Run("cd_with_path", func(t *testing.T) {
		paths := ExtractPaths("cd", []string{"/tmp"})
		if len(paths) != 1 || paths[0] != "/tmp" {
			t.Errorf("expected [/tmp], got %v", paths)
		}
	})

	t.Run("ls_default_cwd", func(t *testing.T) {
		// Source: pathValidation.ts:198-201
		paths := ExtractPaths("ls", []string{"-la"})
		if len(paths) != 1 || paths[0] != "." {
			t.Errorf("expected [.], got %v", paths)
		}
	})

	t.Run("ls_with_paths", func(t *testing.T) {
		paths := ExtractPaths("ls", []string{"-la", "/tmp", "/var"})
		if len(paths) != 2 {
			t.Fatalf("expected 2 paths, got %d: %v", len(paths), paths)
		}
	})

	t.Run("rm_filters_flags", func(t *testing.T) {
		// Source: pathValidation.ts:274
		paths := ExtractPaths("rm", []string{"-rf", "file.txt", "dir/"})
		if len(paths) != 2 || paths[0] != "file.txt" || paths[1] != "dir/" {
			t.Errorf("expected [file.txt, dir/], got %v", paths)
		}
	})

	t.Run("find_collects_before_flags", func(t *testing.T) {
		// Source: pathValidation.ts:211-269
		paths := ExtractPaths("find", []string{".", "-name", "*.go"})
		if len(paths) != 1 || paths[0] != "." {
			t.Errorf("expected [.], got %v", paths)
		}
	})

	t.Run("find_with_path_flag", func(t *testing.T) {
		// Source: pathValidation.ts:253-259
		paths := ExtractPaths("find", []string{".", "-newer", "ref.txt"})
		if len(paths) != 2 {
			t.Fatalf("expected 2 paths, got %d: %v", len(paths), paths)
		}
		if paths[0] != "." || paths[1] != "ref.txt" {
			t.Errorf("expected [., ref.txt], got %v", paths)
		}
	})

	t.Run("find_default_cwd", func(t *testing.T) {
		// Source: pathValidation.ts:268
		paths := ExtractPaths("find", []string{"-name", "*.go"})
		if len(paths) != 1 || paths[0] != "." {
			t.Errorf("expected [.], got %v", paths)
		}
	})

	t.Run("cat_filters_flags", func(t *testing.T) {
		paths := ExtractPaths("cat", []string{"-n", "file.txt"})
		if len(paths) != 1 || paths[0] != "file.txt" {
			t.Errorf("expected [file.txt], got %v", paths)
		}
	})

	t.Run("unknown_command_filters_flags", func(t *testing.T) {
		paths := ExtractPaths("unknown", []string{"-x", "arg1", "arg2"})
		if len(paths) != 2 {
			t.Errorf("expected 2 paths, got %d: %v", len(paths), paths)
		}
	})
}

func TestResolvePath(t *testing.T) {
	t.Run("absolute_unchanged", func(t *testing.T) {
		if p := ResolvePath("/tmp/file", "/home"); p != "/tmp/file" {
			t.Errorf("got %q", p)
		}
	})

	t.Run("relative_joined", func(t *testing.T) {
		p := ResolvePath("src/main.go", "/home/user/project")
		if p != "/home/user/project/src/main.go" {
			t.Errorf("got %q", p)
		}
	})
}

func TestIsPathInDirectory(t *testing.T) {
	t.Run("inside", func(t *testing.T) {
		if !IsPathInDirectory("/home/user/project/src", "/home/user/project") {
			t.Error("should be inside")
		}
	})

	t.Run("outside", func(t *testing.T) {
		if IsPathInDirectory("/etc/passwd", "/home/user/project") {
			t.Error("should be outside")
		}
	})

	t.Run("exact_match", func(t *testing.T) {
		if !IsPathInDirectory("/home/user/project", "/home/user/project") {
			t.Error("exact match should be inside")
		}
	})
}
