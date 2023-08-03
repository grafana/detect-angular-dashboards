//go:build mage

package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v53/github"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

type Build mg.Namespace

const (
	distFolder      = "dist"
	artifactsFolder = "artifacts"

	droneServerURL = "https://drone.grafana.net"
	gitHubOrg      = "grafana"
	gitHubRepo     = "detect-angular-dashboards"
	droneRepo      = gitHubOrg + "/" + gitHubRepo
)

// Go builds the go binary for the specified os and arch into dist/<os>_<arch>/detect-angular-dashboards.
func (Build) Go(goOs, goArch string) error {
	fmt.Println("building for", goOs, goArch)

	const programName = "detect-angular-dashboards"
	args := []string{"build", "-o", filepath.Join(distFolder, goOs+"_"+goArch, programName)}

	ldFlags := []string{"-s", "-w"}
	const buildPkg = "github.com/grafana/detect-angular-dashboards/build"

	if commitSha := gitCommitSha(); commitSha != "" {
		// If commit sha was determined, add it to ldflags
		ldFlags = append(ldFlags, fmt.Sprintf("-X %s.LinkerCommitSHA=%s", buildPkg, commitSha))
	}
	if droneTag := os.Getenv("DRONE_TAG"); droneTag != "" {
		// Add drone tag as linker version
		ldFlags = append(ldFlags, fmt.Sprintf("-X %s.LinkerVersion=%s", buildPkg, droneTag))
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
	return sh.RunV("docker", "build", "-t", "detect-angular-dashboards:"+tag, ".")
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
		zipFn := filepath.Join(outFolder, fmt.Sprintf("%s_%s_%s.zip", gitHubRepo, releaseName, d.Name()))

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
	return sh.RunV("go", "test", "./...")
}

// Lint runs golangci-lint.
func Lint() error {
	if err := sh.RunV("golangci-lint", "run", "./..."); err != nil {
		return err
	}

	return nil
}

// Drone runs drone lint to ensure .drone.yml is valid and it signs the Drone configuration file.
// This needs to be run everytime the .drone.yml file is modified.
// See https://github.com/grafana/deployment_tools/blob/master/docs/infrastructure/drone/signing.md for more info
func Drone() error {
	if err := sh.RunV("drone", "lint", "--trusted"); err != nil {
		return err
	}
	if err := sh.RunV("drone", "--server", droneServerURL, "sign", "--save", droneRepo); err != nil {
		return err
	}
	return nil
}

type GitHub mg.Namespace

// Release pushes a GitHub release
func (g GitHub) Release(releaseName string) error {
	mg.Deps(mg.F(Package, releaseName))

	// Determine files to upload
	artifactsRoot := filepath.Join(distFolder, artifactsFolder, releaseName)
	toUploadFileNames := map[string]struct{}{}
	if err := filepath.WalkDir(artifactsRoot, func(path string, d fs.DirEntry, err error) error {
		if path == artifactsRoot {
			// Skip folder itself
			return nil
		}
		if d.IsDir() {
			// Do not recurse
			return filepath.SkipDir
		}
		if filepath.Ext(d.Name()) != ".zip" {
			// Skip non-zip files
			return nil
		}
		toUploadFileNames[filepath.Join(artifactsRoot, d.Name())] = struct{}{}
		return nil
	}); err != nil {
		return fmt.Errorf("walkdir: %w", err)
	}

	// Ensure we have files to attach to the release
	if len(toUploadFileNames) == 0 {
		return fmt.Errorf("could not find artifacts to upload for %q", releaseName)
	}

	// Check and get GitHub app env vars
	var ghAppID, ghInstallationID int64
	for _, o := range []struct {
		dst    *int64
		envVar string
	}{
		{&ghAppID, "GITHUB_APP_ID"},
		{&ghInstallationID, "GITHUB_APP_INSTALLATION_ID"},
	} {
		var err error
		v := os.Getenv(o.envVar)
		*o.dst, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			return fmt.Errorf("%q (value of env var %q) is not an integer", v, o.envVar)
		}
	}

	// Create GitHub client
	ghTransport, err := ghinstallation.New(http.DefaultTransport, ghAppID, ghInstallationID, []byte(os.Getenv("GITHUB_APP_PRIVATE_KEY")))
	if err != nil {
		return fmt.Errorf("ghinstallation new: %w", err)
	}
	ghClient := github.NewClient(&http.Client{Transport: ghTransport})
	ctx, canc := context.WithTimeout(context.Background(), time.Minute*10)
	defer canc()

	// Create release
	release, _, err := ghClient.Repositories.CreateRelease(ctx, gitHubOrg, gitHubRepo, &github.RepositoryRelease{
		Name:       github.String(releaseName),
		TagName:    github.String(releaseName),
		Draft:      github.Bool(false),
		Prerelease: github.Bool(false),
		MakeLatest: github.String("true"),
	})
	if err != nil {
		return fmt.Errorf("create github release: %w", err)
	}
	fmt.Println("created github release", releaseName)

	// Set up error handling
	var finalErr error
	errs := make(chan error)
	go func() {
		for err := range errs {
			finalErr = errors.Join(finalErr, err)
		}
	}()

	// Upload all artifacts and attach them to the release
	var wg sync.WaitGroup
	wg.Add(len(toUploadFileNames))
	for fn := range toUploadFileNames {
		fn := fn
		go func() {
			defer wg.Done()

			fmt.Println("uploading", fn, "...")
			f, err := os.Open(fn)
			if err != nil {
				errs <- fmt.Errorf("open %q: %w", fn, err)
				return
			}
			defer func() {
				if err := f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
					errs <- fmt.Errorf("close %q: %w", fn, err)
				}
			}()

			if _, _, err := ghClient.Repositories.UploadReleaseAsset(ctx, gitHubOrg, gitHubRepo, *release.ID, &github.UploadOptions{
				Name: filepath.Base(fn),
			}, f); err != nil {
				errs <- fmt.Errorf("upload release artifact %q: %w", fn, err)
				return
			}
			fmt.Println("upload", fn, "ok!")
		}()
	}

	// Wait for upload goroutines to finish
	wg.Wait()
	close(errs)
	return finalErr
}

// gitCommitSha returns the git commit sha for the current repo or "" if none.
// It tries to get it from DRONE_COMMIT_SHA env var (set from drone).
// If it's not set, it invokes `git`.
// If it's not possible to run `git`, it returns an empty string.
func gitCommitSha() string {
	// Try to get git commit sha, prioritize env var from drone
	if commitSha := os.Getenv("DRONE_COMMIT_SHA"); commitSha != "" {
		return commitSha
	}
	// If not possible, try invoking `git` command
	hash, _ := sh.Output("git", "rev-parse", "--short", "HEAD")
	return hash
}
