package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/sumdb/dirhash"
)

// rsc.io/quote is a tiny, well-known Go module used in official Go tutorials.
const (
	testModule  = "rsc.io/quote"
	testVersion = "v1.5.2"
	testRepoURL = "https://github.com/rsc/quote"
)

// TestZipMatchesProxy generates a module zip with the tool and compares its
// dirhash against the zip served by proxy.golang.org.
//
// Environment variables:
//   - GOMODCACHE: if set (or auto-detected), its cache/download tree is checked
//     for a pre-downloaded reference zip before hitting the network.
//   - GOPACKER_TEST_GITCACHE: directory where the test git clone is cached
//     across runs to avoid repeated network clones.
func TestZipMatchesProxy(t *testing.T) {
	repoDir := getOrCloneRepo(t)

	outDir := t.TempDir()
	cmd := exec.Command("go", "run", ".",
		"-repo", repoDir,
		"-version", testVersion,
		"-out", outDir,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tool failed: %v\n%s", err, out)
	}

	ourZip := filepath.Join(outDir, testVersion+".zip")
	ourHash, err := dirhash.HashZip(ourZip, dirhash.Hash1)
	if err != nil {
		t.Fatalf("failed to hash generated zip: %v", err)
	}

	refZip := getReferenceZip(t)
	refHash, err := dirhash.HashZip(refZip, dirhash.Hash1)
	if err != nil {
		t.Fatalf("failed to hash reference zip: %v", err)
	}

	if ourHash != refHash {
		t.Errorf("zip content hash mismatch:\n  got  %s\n  want %s", ourHash, refHash)
	}
}

// getOrCloneRepo returns a path to a git clone of rsc.io/quote at testVersion.
// Set GOPACKER_TEST_GITCACHE to a directory to persist the clone across test runs.
func getOrCloneRepo(t *testing.T) string {
	t.Helper()

	if cacheDir := os.Getenv("GOPACKER_TEST_GITCACHE"); cacheDir != "" {
		repoDir := filepath.Join(cacheDir, "rsc-quote-"+testVersion)
		if _, err := os.Stat(filepath.Join(repoDir, ".git")); err == nil {
			return repoDir
		}
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			t.Fatal(err)
		}
		cloneRepo(t, repoDir)
		return repoDir
	}

	repoDir := filepath.Join(t.TempDir(), "rsc-quote")
	cloneRepo(t, repoDir)
	return repoDir
}

func cloneRepo(t *testing.T, dest string) {
	t.Helper()
	cmd := exec.Command("git", "clone", "--depth=1", "--branch", testVersion, testRepoURL, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone failed: %v\n%s", err, out)
	}
}

func TestCheckDirExists(t *testing.T) {
	t.Run("nonexistent path", func(t *testing.T) {
		exists, err := checkDirExists(filepath.Join(t.TempDir(), "no-such-dir"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if exists {
			t.Error("expected false for nonexistent path")
		}
	})

	t.Run("existing directory", func(t *testing.T) {
		exists, err := checkDirExists(t.TempDir())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !exists {
			t.Error("expected true for existing directory")
		}
	})

	t.Run("path is a file", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "file-*")
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		exists, err := checkDirExists(f.Name())
		if err == nil {
			t.Fatal("expected error for file path, got nil")
		}
		if !strings.Contains(err.Error(), "not a directory") {
			t.Errorf("expected 'not a directory' in error, got: %v", err)
		}
		if exists {
			t.Error("expected false when path is a file")
		}
	})
}

func TestHandleOutputDir(t *testing.T) {
	t.Run("explicit dir that already exists", func(t *testing.T) {
		dir := t.TempDir()
		got, err := handleOutputDir(dir, "example.com/mod")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != dir {
			t.Errorf("got %q, want %q", got, dir)
		}
	})

	t.Run("explicit dir that does not exist", func(t *testing.T) {
		newDir := filepath.Join(t.TempDir(), "new", "nested", "dir")
		got, err := handleOutputDir(newDir, "example.com/mod")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != newDir {
			t.Errorf("got %q, want %q", got, newDir)
		}
		if _, err := os.Stat(newDir); err != nil {
			t.Errorf("expected directory to be created: %v", err)
		}
	})

	t.Run("explicit path that is a file", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "file-*")
		if err != nil {
			t.Fatal(err)
		}
		f.Close()

		_, err = handleOutputDir(f.Name(), "example.com/mod")
		if err == nil {
			t.Fatal("expected error when output path is a file, got nil")
		}
	})

	t.Run("auto-detect via GOMODCACHE env", func(t *testing.T) {
		fakeCache := t.TempDir()
		t.Setenv("GOMODCACHE", fakeCache)

		modulePath := "example.com/mymod"
		got, err := handleOutputDir("", modulePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		wantSuffix := filepath.Join("cache", "download", modulePath, "@v")
		if !strings.HasSuffix(got, wantSuffix) {
			t.Errorf("got %q, expected suffix %q", got, wantSuffix)
		}
		if _, err := os.Stat(got); err != nil {
			t.Errorf("expected output directory to be created: %v", err)
		}
	})
}

// getReferenceZip returns a path to the official module zip for testModule at testVersion.
// Checks $GOMODCACHE/cache/download first, then downloads from proxy.golang.org.
func getReferenceZip(t *testing.T) string {
	t.Helper()

	gomodcache := os.Getenv("GOMODCACHE")
	if gomodcache == "" {
		if out, err := exec.Command("go", "env", "GOMODCACHE").Output(); err == nil {
			gomodcache = strings.TrimSpace(string(out))
		}
	}

	if gomodcache != "" {
		// module path segments: rsc.io/quote -> rsc.io / quote
		parts := strings.Split(testModule, "/")
		cachePath := append([]string{gomodcache, "cache", "download"}, parts...)
		cachePath = append(cachePath, "@v", testVersion+".zip")
		cached := filepath.Join(cachePath...)
		if _, err := os.Stat(cached); err == nil {
			return cached
		}
	}

	// Fall back to downloading from the module proxy.
	url := fmt.Sprintf("https://proxy.golang.org/%s/@v/%s.zip", testModule, testVersion)
	resp, err := http.Get(url)
	if err != nil {
		t.Skipf("skipping: cannot reach proxy.golang.org (%v)", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("skipping: proxy returned HTTP %d for %s", resp.StatusCode, url)
	}

	f, err := os.CreateTemp(t.TempDir(), "ref-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}
