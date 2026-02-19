package selfupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindBinary(t *testing.T) {
	tmpDir := t.TempDir()

	// Helper to create file
	createFile := func(path string, mode os.FileMode) {
		dir := filepath.Dir(path)
		os.MkdirAll(dir, 0755)
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		os.Chmod(path, mode)
	}

	t.Run("Exact match workspaced", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "exact")
		os.MkdirAll(dir, 0755)
		createFile(filepath.Join(dir, "workspaced"), 0755)

		found, err := findBinary(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(found) != "workspaced" {
			t.Errorf("expected workspaced, got %s", found)
		}
	})

	t.Run("Exact match workspaced.exe", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "exact_exe")
		os.MkdirAll(dir, 0755)
		createFile(filepath.Join(dir, "workspaced.exe"), 0755)

		found, err := findBinary(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(found) != "workspaced.exe" {
			t.Errorf("expected workspaced.exe, got %s", found)
		}
	})

	t.Run("Match in bin/", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "in_bin")
		os.MkdirAll(dir, 0755)
		createFile(filepath.Join(dir, "bin", "workspaced"), 0755)

		found, err := findBinary(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(found) != "workspaced" {
			t.Errorf("expected workspaced, got %s", found)
		}
	})

	t.Run("Fallback scan single binary", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "fallback_single")
		os.MkdirAll(dir, 0755)

		binName := "workspaced-custom"
		if runtime.GOOS == "windows" {
			binName += ".exe"
		}
		createFile(filepath.Join(dir, binName), 0755)
		createFile(filepath.Join(dir, "README.md"), 0644)

		found, err := findBinary(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if filepath.Base(found) != binName {
			t.Errorf("expected %s, got %s", binName, found)
		}
	})

	// Platform specific fallback tests
	if runtime.GOOS != "windows" {
		t.Run("Fallback scan executable bit", func(t *testing.T) {
			dir := filepath.Join(tmpDir, "fallback_exec")
			os.MkdirAll(dir, 0755)

			createFile(filepath.Join(dir, "not_exec"), 0644)
			createFile(filepath.Join(dir, "is_exec"), 0755) // +x

			found, err := findBinary(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if filepath.Base(found) != "is_exec" {
				t.Errorf("expected is_exec, got %s", found)
			}
		})
	} else {
		t.Run("Fallback scan exe extension", func(t *testing.T) {
			dir := filepath.Join(tmpDir, "fallback_exe_ext")
			os.MkdirAll(dir, 0755)

			createFile(filepath.Join(dir, "not_exe"), 0755)
			createFile(filepath.Join(dir, "is_exe.exe"), 0755)

			found, err := findBinary(dir)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if filepath.Base(found) != "is_exe.exe" {
				t.Errorf("expected is_exe.exe, got %s", found)
			}
		})
	}

	t.Run("No binary found", func(t *testing.T) {
		dir := filepath.Join(tmpDir, "none")
		os.MkdirAll(dir, 0755)
		createFile(filepath.Join(dir, "README.md"), 0644)

		_, err := findBinary(dir)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
