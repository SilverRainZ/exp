// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vulncheck

import (
	"context"
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/vuln/client"
)

// modKey creates a unique string identifier for mod.
func modKey(mod *Module) string {
	if mod == nil {
		return ""
	}
	return fmt.Sprintf("%s@%s", mod.Path, mod.Version)
}

// extractModules collects modules in `pkgs` up to uniqueness of
// module path and version.
func extractModules(pkgs []*Package) []*Module {
	modMap := map[string]*Module{}

	seen := map[*Package]bool{}
	var extract func(*Package, map[string]*Module)
	extract = func(pkg *Package, modMap map[string]*Module) {
		if pkg == nil || seen[pkg] {
			return
		}
		if pkg.Module != nil {
			if pkg.Module.Replace != nil {
				modMap[modKey(pkg.Module.Replace)] = pkg.Module
			} else {
				modMap[modKey(pkg.Module)] = pkg.Module
			}
		}
		seen[pkg] = true
		for _, imp := range pkg.Imports {
			extract(imp, modMap)
		}
	}
	for _, pkg := range pkgs {
		extract(pkg, modMap)
	}

	modules := []*Module{}
	for _, mod := range modMap {
		modules = append(modules, mod)
	}
	return modules
}

// fetchVulnerabilities fetches vulnerabilities that affect the supplied modules.
func fetchVulnerabilities(ctx context.Context, client client.Client, modules []*Module) (moduleVulnerabilities, error) {
	mv := moduleVulnerabilities{}
	for _, mod := range modules {
		modPath := mod.Path
		if mod.Replace != nil {
			modPath = mod.Replace.Path
		}

		// skip loading vulns for local imports
		if isLocal(mod) {
			// TODO: what if client has its own db
			// with local vulns?
			continue
		}
		vulns, err := client.GetByModule(ctx, modPath)
		if err != nil {
			return nil, err
		}
		if len(vulns) == 0 {
			continue
		}
		mv = append(mv, modVulns{
			mod:   mod,
			vulns: vulns,
		})
	}
	return mv, nil
}

// fetchingInTesting is a flag used to avoid skipping
// loading local vulnerabilities in testing.
var fetchingInTesting bool = false

func isLocal(mod *Module) bool {
	if fetchingInTesting {
		return false
	}
	modDir := mod.Dir
	if mod.Replace != nil {
		modDir = mod.Replace.Dir
	}
	return modDir != "" && !strings.HasPrefix(modDir, modCacheDirectory())
}
func modCacheDirectory() string {
	var modCacheDir string
	// TODO: define modCacheDir using something similar to cmd/go/internal/cfg.GOMODCACHE?
	if modCacheDir = os.Getenv("GOMODCACHE"); modCacheDir == "" {
		if modCacheDir = os.Getenv("GOPATH"); modCacheDir == "" {
			modCacheDir = build.Default.GOPATH
		}
		modCacheDir = filepath.Join(modCacheDir, "pkg", "mod")
	}
	return modCacheDir
}
