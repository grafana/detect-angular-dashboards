//go:build mage

package main

import (
	"archive/zip"
	"fmt"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

type Build mg.Namespace

const (
	distFolder      = "dist"
	artifactsFolder = "artifacts"

	droneServerURL = "https://drone.grafana.net"
	droneRepo      = "github.com/grafana/detect-angular-dashboards"
)

// Go builds the go binary for the specified os and arch into dist/<os>_<arch>/detect-angular-dashboards.
func (Build) Go(goOs, goArch string) error {
	const programName = "detect-angular-dashboards"
	fmt.Println("building for", goOs, goArch)
	return sh.RunWithV(
		map[string]string{
			"CGO_ENABLED": "0",
			"GOOS":        goOs,
			"GOARCH":      goArch,
		},
		"go", "build", "-v", "-o", filepath.Join(distFolder, goOs+"_"+goArch, programName),
	)
}

// Build builds the binary for the current os and arch.
func (b Build) Build() error {
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

func (b Build) Package(releaseName string) error {
	mg.Deps(b.All)
	return filepath.WalkDir(distFolder, func(path string, d fs.DirEntry, err error) error {
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
		zipFn := filepath.Join(outFolder, fmt.Sprintf("%s_%s.zip", d.Name(), releaseName))
		fmt.Println("creating release package", zipFn)
		if err := b.zipFolder(path, zipFn); err != nil {
			return fmt.Errorf("zip folder %q: %w", zipFn, err)
		}
		return nil
	})
}

// Docker builds the docker image with the specified tag.
func (Build) Docker(tag string) error {
	return sh.RunV("docker", "build", "-t", "detect-angular-dashboards:"+tag, ".")
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
	mg.Deps(mg.F(Build{}.Package, releaseName))
	// TODO: create GitHub release
	return nil
}
