//go:build mage

package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

type Build mg.Namespace

const (
	distFolder      = "dist"
	artifactsFolder = "artifacts"
	programName     = "detect-angular-dashboards"
)

// Go builds the go binary for the specified os and arch into dist/<os>_<arch>/detect-angular-dashboards.
func (Build) Go(goOs, goArch string) error {
	fmt.Println("building for", goOs, goArch)

	args := []string{"build", "-o", filepath.Join(distFolder, goOs+"_"+goArch, programName)}

	ldFlags := []string{"-s", "-w"}
	const buildPkg = "github.com/grafana/detect-angular-dashboards/build"

	if commitSha := gitCommitSha(); commitSha != "" {
		// If commit sha was determined, add it to ldflags
		ldFlags = append(ldFlags, fmt.Sprintf("-X %s.LinkerCommitSHA=%s", buildPkg, commitSha))
	}
	if version, err := releasePleaseVersion(); err == nil && version != "" {
		// Add release please version as linker version
		ldFlags = append(ldFlags, fmt.Sprintf("-X %s.LinkerVersion=%s", buildPkg, version))
	}

	// Add all ldflags to args
	if len(ldFlags) > 0 {
		args = append(args, "-ldflags", strings.Join(ldFlags, " "))
	}

	// Run `go build` command
	return sh.RunWithV(
		map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        goOs,
			"GOARCH":      goArch,
		},
		"go",
		args...,
	)
}

// Current builds the binary for the current os and arch.
func (b Build) Current() error {
	return b.Go(runtime.GOOS, runtime.GOARCH)
}

// All builds all supported binaries into the dist folder.
func (b Build) All() error {
	oses := []string{"linux", "darwin", "windows"}
	archs := []string{"amd64", "arm64"}
	var deps []interface{}
	for _, os := range oses {
		for _, arch := range archs {
			deps = append(deps, mg.F(b.Go, os, arch))
		}
	}
	mg.Deps(deps...)
	return nil
}

// zipFolder zips all files in inFolder into the outFileName .zip file (which will be created).
func (b Build) zipFolder(inFolder string, outFileName string) error {
	w, err := os.Create(outFileName)
	if err != nil {
		return fmt.Errorf("%q zip create: %w", outFileName, err)
	}
	defer w.Close()

	zw := zip.NewWriter(w)
	defer zw.Close()
	if err := filepath.WalkDir(inFolder, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("%q open: %w", path, err)
		}
		defer f.Close()

		relFn, err := filepath.Rel(inFolder, path)
		if err != nil {
			return fmt.Errorf("filepath rel %q: %w", path, err)
		}
		zfw, err := zw.Create(relFn)
		if err != nil {
			return fmt.Errorf("%q create: %w", relFn, err)
		}

		if _, err := io.Copy(zfw, f); err != nil {
			return fmt.Errorf("io copy: %w", err)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("walkdir: %w", err)
	}
	return nil
}

// Docker builds the docker image with the specified tag.
func (Build) Docker(tag string) error {
	return sh.RunV("docker", "build", "-t", programName+":"+tag, ".")
}

// Package runs build:all and creates multiple .zip files inside dist/artifacts/<releaseName>, one for each folder in dist/*.
func Package(releaseName string) error {
	var b Build
	mg.Deps(b.All)

	// Join all zip and general walkdir error
	errs := make(chan error)
	var finalErr error
	go func() {
		for err := range errs {
			finalErr = errors.Join(finalErr, err)
		}
	}()

	var wg sync.WaitGroup
	errs <- filepath.WalkDir(distFolder, func(path string, d fs.DirEntry, err error) error {
		// Skip dist folder (first call)
		if path == distFolder {
			return nil
		}
		// Recursively skip artifacts folder (output folder)
		if d.Name() == artifactsFolder {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			return nil
		}
		outFolder := filepath.Join(distFolder, artifactsFolder, releaseName)
		if err := os.MkdirAll(outFolder, os.ModePerm); err != nil {
			return fmt.Errorf("mkdir %q: %w", outFolder, err)
		}
		zipFn := filepath.Join(outFolder, fmt.Sprintf("%s_%s_%s.zip", programName, releaseName, d.Name()))

		wg.Add(1)
		go func() {
			fmt.Println("creating release package", zipFn)
			if err := b.zipFolder(path, zipFn); err != nil {
				errs <- fmt.Errorf("zip folder %q: %w", zipFn, err)
			}
			wg.Done()
		}()

		return nil
	})

	// Wait for everyone to terminate
	wg.Wait()
	close(errs)
	return finalErr
}

// Clean deletes all the build binaries and artifacts from dist.
func Clean() error {
	if err := os.RemoveAll(distFolder); err != nil {
		return fmt.Errorf("removeall: %w", err)
	}
	if err := os.MkdirAll(distFolder, os.ModePerm); err != nil {
		return fmt.Errorf("mkdirall: %w", err)
	}
	return nil
}

// Test runs the test suite.
func Test() error {
	return sh.RunV("go", "test", "-v", "./...")
}

// Lint runs golangci-lint.
func Lint() error {
	if err := sh.RunV("golangci-lint", "run", "./..."); err != nil {
		return err
	}

	return nil
}

// gitCommitSha returns the git commit sha for the current repo or "" if none.
// It invokes `git` to determine the commit sha.
// If it's not possible to run `git`, it returns an empty string.
func gitCommitSha() string {
	hash, _ := sh.Output("git", "rev-parse", "--short", "HEAD")
	return hash
}

// releasePleaseVersion reads the .release-please-manifest.json file and returns the version for the current component (".").
func releasePleaseVersion() (string, error) {
	f, err := os.Open(".release-please-manifest.json")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	var manifest map[string]string
	if err := json.NewDecoder(f).Decode(&manifest); err != nil {
		return "", err
	}
	const releasePleaseComponent = "."
	return manifest[releasePleaseComponent], nil
}
