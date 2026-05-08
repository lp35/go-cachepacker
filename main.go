package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/zip"
)

func main() {
	repoPath := flag.String("repo", "", "Path to the git repository (absolute or relative)")
	version := flag.String("version", "", "Module version: either a tag (e.g. v1.2.3) or a pseudo-version (e.g. v0.0.0-20260510095722-d4e5f6a7b8c9). The git ref is derived automatically is pseudo-version is detected")
	outDir := flag.String("out", "", "Output directory for the zip file (optional). Default to ${GOMODCACHE}/cache/download/<MODULE_NAME>/@v")

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `go-cachepacker - creates a Go module ZIP from a local VCS repository,
suitable for use with a Go module proxy or cache.

The git repository does not need to be checked out at the target version:
the tool resolves the correct commit internally from the version string,
and does not touch the state of the target repository.

This is particularly useful when a Go dependency lives in a private repository:
  - It avoids the need for a "url.insteadOf" git rewrite rule, which can leak
    credentials into the global git config.
  - It removes the need for a "replace" directive in go.mod, which would bypass
    the Go module proxy and checksum verification (go.sum).
  - The generated ZIP produces the same hash as the one a Go proxy would serve, so "go get"
    can still verify the module checksum as usual.

Usage:
  go-cachepacker -repo <path> [-version <version>] [-out <dir>]

Flags:
`)
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), `
Example:
  go-cachepacker -repo /path/to/mymodule -version v1.2.3
`)
	}

	flag.Parse()

	if *repoPath == "" {
		fmt.Fprintln(os.Stderr, "error: -repo flag is required")
		flag.Usage()
		os.Exit(1)
	}

	if *version == "" {
		fmt.Fprintln(os.Stderr, "error: -version flag is required")
		flag.Usage()
		os.Exit(1)
	}

	absRepoPath, err := filepath.Abs(*repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not resolve repo path: %v\n", err)
		os.Exit(1)
	}

	gomodData, err := os.ReadFile(filepath.Join(absRepoPath, "go.mod"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not read go.mod from repo: %v\n", err)
		os.Exit(1)
	}
	modPath := modfile.ModulePath(gomodData)
	if modPath == "" {
		fmt.Fprintln(os.Stderr, "error: could not determine module path from go.mod")
		os.Exit(1)
	}

	*outDir, err = handleOutputDir(*outDir, modPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: could not process output dir: %w", err)
		os.Exit(1)
	}

	var gitRef string
	if rev, err := module.PseudoVersionRev(*version); err == nil {
		gitRef = rev
	} else {
		gitRef = *version
	}

	mod := module.Version{
		Path:    modPath,
		Version: *version,
	}

	filename := filepath.Join(*outDir, fmt.Sprintf("%s.zip", mod.Version))
	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	err = zip.CreateFromVCS(
		f,
		mod,
		absRepoPath,
		gitRef,
		"",
	)
	if err != nil {
		panic(err)
	}
}

func handleOutputDir(outputDir, modulePath string) (string, error) {
	if outputDir == "" {
		out, err := exec.Command("go", "env", "GOMODCACHE").Output()
		if err == nil {
			outputDir = strings.TrimSpace(string(out))
			outputDir = path.Join(outputDir, `/cache/download/`, modulePath, `/@v`)
		} else {
			// Use current folder by default
			return ".", nil
		}
	}

	exists, err := checkDirExists(outputDir)
	if err != nil {
		return "", fmt.Errorf("error: issue with directory %q: %w", outputDir, err)
	}

	if !exists {
		err = os.MkdirAll(outputDir, 0775)
		if err != nil {
			return "", fmt.Errorf("error: can't create directory %q: %w", outputDir, err)
		}
	}

	return outputDir, nil
}

func checkDirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	if !info.IsDir() {
		return false, fmt.Errorf("path exists but is not a directory")
	}

	return true, nil
}
