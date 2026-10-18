// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

//go:build mage

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/magefile/mage/mg"
)

const (
	PACKAGE = `code.vikunja.io/api`
	DIST    = `dist`
)

var (
	Executable    = "vikunja"
	Ldflags       = ""
	Tags          = ""
	VersionNumber = "dev"
	Version       = "unstable" // This holds the built version, unstable by default, when building from a tag or release branch, their name
	BinLocation   = ""
	PkgVersion    = "unstable"

	// Aliases are mage aliases of targets
	Aliases = map[string]any{
		"build":                  Build.Build,
		"check:got-swag":         Check.GotSwag,
		"dev:make-migration":     Dev.MakeMigration,
		"dev:make-event":         Dev.MakeEvent,
		"dev:make-listener":      Dev.MakeListener,
		"dev:make-notification":  Dev.MakeNotification,
		"dev:prepare-worktree":   Dev.PrepareWorktree,
		"test:e2e-api":           Test.E2EApi,
		"plugins:build":          Plugins.Build,
		"lint":                   Check.Golangci,
		"lint:fix":               Check.GolangciFix,
		"generate:config-yaml":   Generate.ConfigYAML,
		"generate:swagger-docs":  Generate.SwaggerDocs,
		"generate:yaegi-symbols": Generate.YaegiSymbols,
		"check:yaegi-symbols":    Check.YaegiSymbols,
	}
)

func goDetectVerboseFlag() string {
	return fmt.Sprintf("-v=%t", mg.Verbose())
}

func runGitCommandWithOutput(ctx context.Context, arg ...string) (output []byte, err error) {
	cmd := exec.CommandContext(ctx, "git", arg...)
	output, err = cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("error running command: %s, %w", string(ee.Stderr), err)
		}
		return nil, fmt.Errorf("error running command: %w", err)
	}

	return output, nil
}

func getRawVersionString(ctx context.Context) (version string, err error) {
	version, err = getRawVersionNumber(ctx)
	if err != nil {
		return
	}

	if version == "main" {
		version = "unstable"
	}

	if version != "" && version != "unstable" {
		return
	}

	return
}

func getRawVersionNumber(ctx context.Context) (version string, err error) {
	versionEnv := os.Getenv("RELEASE_VERSION")
	if versionEnv != "" {
		return versionEnv, nil
	}

	if os.Getenv("DRONE_TAG") != "" {
		return os.Getenv("DRONE_TAG"), nil
	}

	if os.Getenv("DRONE_BRANCH") != "" {
		return strings.Replace(os.Getenv("DRONE_BRANCH"), "release/v", "", 1), nil
	}

	versionBytes, err := runGitCommandWithOutput(ctx, "describe", "--tags", "--always", "--abbrev=10")
	return string(versionBytes), err
}

func setVersion(ctx context.Context) error {
	versionNumber, err := getRawVersionNumber(ctx)
	if err != nil {
		return err
	}
	VersionNumber = strings.Trim(versionNumber, "\n")
	VersionNumber = strings.Replace(VersionNumber, "-g", "-", 1)

	version, err := getRawVersionString(ctx)
	if err != nil {
		return fmt.Errorf("error getting version: %w", err)
	}
	Version = version
	return nil
}

func setBinLocation() {
	if os.Getenv("DRONE_WORKSPACE") != "" {
		BinLocation = DIST + `/binaries/` + Executable + `-` + Version + `-linux-amd64`
	} else {
		BinLocation = Executable
	}
}

func setPkgVersion() {
	if Version == "unstable" {
		PkgVersion = VersionNumber
	}
}

func setExecutable() {
	if runtime.GOOS == "windows" {
		Executable += ".exe"
	}
}

// Some variables can always get initialized, so we do just that.
func init() {
	setExecutable()
}

// Some variables have external dependencies (like git) which may not always be available.
func initVars(ctx context.Context) error {
	// Always include osusergo to use pure Go os/user implementation instead of CGO.
	// This prevents SIGFPE crashes when running under systemd without HOME set,
	// caused by glibc's getpwuid_r() failing in certain environments.
	// See: https://github.com/go-vikunja/vikunja/issues/2170
	Tags = "osusergo " + strings.ReplaceAll(os.Getenv("TAGS"), ",", " ")
	if err := setVersion(ctx); err != nil {
		return err
	}
	setBinLocation()
	setPkgVersion()
	Ldflags = `-X "` + PACKAGE + `/pkg/version.Version=` + VersionNumber + `" -X "main.Tags=` + Tags + `"`
	return nil
}

func runAndStreamOutput(ctx context.Context, cmd string, args ...string) error {
	c := exec.CommandContext(ctx, cmd, args...)

	c.Env = os.Environ()
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	fmt.Printf("%s\n\n", c.String())
	return c.Run()
}

// Will check if the tool exists and if not install it from the provided import path
// If any errors occur, it will exit with a status code of 1.
func checkAndInstallGoTool(ctx context.Context, tool, importPath string) error {
	if err := exec.CommandContext(ctx, tool).Run(); err != nil && strings.Contains(err.Error(), "executable file not found") {
		fmt.Printf("%s not installed, installing %s...\n", tool, importPath)
		if err := exec.CommandContext(ctx, "go", "install", goDetectVerboseFlag(), importPath).Run(); err != nil { //nolint:gosec // Every caller to checkAndInstallGoTool is hard-coded at time of writing, so no injection possible.
			return fmt.Errorf("error installing %s: %w", tool, err)
		}
		fmt.Println("Installed.")
	}
	return nil
}

// Calculates a hash of a file
func calculateSha256FileHash(path string) (hash string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Copy the src file to dst. Any existing file will be overwritten and will not
// copy file attributes.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	si, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.Chmod(dst, si.Mode()); err != nil {
		return err
	}

	return out.Close()
}

func appendToFile(filename, content string) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}

	defer f.Close()

	_, err = f.WriteString(content)
	return err
}

const InfoColor = "\033[1;32m%s\033[0m"

func printSuccess(text string, args ...any) {
	text = fmt.Sprintf(text, args...)
	fmt.Printf(InfoColor+"\n", text)
}

// getRandomPort finds a random available TCP port.
func getRandomPort(ctx context.Context) (int, error) {
	l, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func ensureFrontendDistExists() error {
	return ensureFrontendDistExistsIn(".")
}

// web/embed.go embeds dist/ with //go:embed all:dist, so every go build and
// go test fails hard when that directory does not exist. A placeholder index.html
// keeps them working in a worktree that never ran a frontend build.
func ensureFrontendDistExistsIn(root string) error {
	distPath := filepath.Join(root, "web", "dist")
	if _, err := os.Stat(distPath); os.IsNotExist(err) {
		if err := os.MkdirAll(distPath, 0o755); err != nil {
			return fmt.Errorf("error creating %s: %w", distPath, err)
		}
	}

	indexFile := filepath.Join(distPath, "index.html")
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		f, err := os.Create(indexFile)
		if err != nil {
			return fmt.Errorf("error creating %s: %w", indexFile, err)
		}
		f.Close()
	}
	return nil
}

// Fmt formats the code using go fmt
func Fmt(ctx context.Context) error {
	mg.Deps(initVars, ensureFrontendDistExists)
	out, err := exec.CommandContext(ctx, "git", "ls-files", "--cached", "--others", "--exclude-standard", "*.go").Output()
	if err != nil {
		return fmt.Errorf("failed to list go files from git: %w", err)
	}
	goFiles := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(goFiles) == 0 || (len(goFiles) == 1 && goFiles[0] == "") {
		return nil
	}
	args := append([]string{"-s", "-w"}, goFiles...)
	return runAndStreamOutput(ctx, "gofmt", args...)
}

type Test mg.Namespace

const webtestsPackage = "./pkg/webtests"

// goTestPackagesExcept expands ./... minus the given package patterns.
func goTestPackagesExcept(ctx context.Context, exclude ...string) ([]string, error) {
	out, err := exec.CommandContext(ctx, "go", "list", "./...").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list go packages: %w", err)
	}

	excluded := make(map[string]bool, len(exclude))
	for _, pattern := range exclude {
		excluded[strings.TrimPrefix(pattern, "./")] = true
	}

	var packages []string
	for _, pkg := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if pkg == "" || excluded[strings.TrimPrefix(pkg, PACKAGE+"/")] {
			continue
		}
		packages = append(packages, pkg)
	}
	return packages, nil
}

// Feature runs the feature tests
func (Test) Feature(ctx context.Context) error {
	mg.Deps(initVars, ensureFrontendDistExists)
	// We run everything sequentially and not in parallel to prevent issues with real test databases
	return runAndStreamOutput(ctx, "go", "test", goDetectVerboseFlag(), "-p", "1", "-coverprofile", "cover.out", "-timeout", "45m", "-short", "./...")
}

// Coverage runs the tests and builds the coverage html file from coverage output
func (Test) Coverage(ctx context.Context) error {
	mg.Deps(initVars)
	mg.Deps(Test.Feature)
	return runAndStreamOutput(ctx, "go", "tool", "cover", "-html=cover.out", "-o", "cover.html")
}

// Web runs the web tests
func (Test) Web(ctx context.Context) error {
	mg.Deps(initVars, ensureFrontendDistExists)
	// We run everything sequentially and not in parallel to prevent issues with real test databases
	args := []string{"test", goDetectVerboseFlag(), "-p", "1", "-timeout", "45m", "./pkg/webtests"}
	return runAndStreamOutput(ctx, "go", args...)
}

// Filter runs every test matching the given `go test -run` filter.
//
// Most packages run with -short, but pkg/webtests is run in a second pass without
// it: its TestMain skips the entire package under -short, so a filter naming a web
// test would otherwise report "ok" without having run anything. The second pass is
// a no-op when the filter matches nothing there.
func (Test) Filter(ctx context.Context, filter string) error {
	mg.Deps(initVars, ensureFrontendDistExists)

	packages, err := goTestPackagesExcept(ctx, webtestsPackage)
	if err != nil {
		return err
	}

	// We run everything sequentially and not in parallel to prevent issues with real test databases
	args := append([]string{"test", goDetectVerboseFlag(), "-p", "1", "-timeout", "45m", "-run", filter, "-short"}, packages...)
	if err := runAndStreamOutput(ctx, "go", args...); err != nil {
		return err
	}

	return runAndStreamOutput(ctx, "go", "test", goDetectVerboseFlag(), "-p", "1", "-timeout", "45m", "-run", filter, webtestsPackage)
}

func (Test) All() {
	mg.Deps(initVars, ensureFrontendDistExists)
	mg.Deps(Test.Feature, Test.Web, Test.Caldav, Test.E2EApi)
}

// Caldav runs the CalDAV protocol compliance tests in pkg/caldavtests.
// These tests exercise the full HTTP router with WebDAV/CalDAV requests.
func (Test) Caldav(ctx context.Context) error {
	mg.Deps(initVars, ensureFrontendDistExists)
	return runAndStreamOutput(ctx, "go", "test", goDetectVerboseFlag(), "-p", "1", "-timeout", "45m", "./pkg/caldavtests")
}

// E2EApi runs the end-to-end API tests in pkg/e2etests.
// These tests use the real event system (not events.Fake()) to verify
// the full async pipeline: web handler → DB → event dispatch → watermill → listener.
func (Test) E2EApi(ctx context.Context) error {
	mg.Deps(initVars, ensureFrontendDistExists)
	return runAndStreamOutput(ctx, "go", "test", goDetectVerboseFlag(), "-p", "1", "-timeout", "45m", "./pkg/e2etests")
}

type Check mg.Namespace

// GotSwag checks if the swagger docs need to be re-generated from the code annotations
func (Check) GotSwag(ctx context.Context) error {
	mg.Deps(initVars)
	// The check is pretty cheaply done: We take the hash of the swagger.json file, generate the docs,
	// hash the file again and compare the two hashes to see if anything changed. If that's the case,
	// regenerating the docs is necessary.
	// swag is not capable of just outputting the generated docs to stdout, therefore we need to do it this way.
	// Another drawback of this is obviously it will only work once - we're not resetting the newly generated
	// docs after the check. This behaviour is good enough for ci though.
	oldHash, err := calculateSha256FileHash("./pkg/swagger/swagger.json")
	if err != nil {
		return fmt.Errorf("error getting old hash of the swagger docs: %w", err)
	}

	if generateErr := (Generate{}).SwaggerDocs(ctx); generateErr != nil {
		return generateErr
	}

	newHash, err := calculateSha256FileHash("./pkg/swagger/swagger.json")
	if err != nil {
		return fmt.Errorf("error getting new hash of the swagger docs: %w", err)
	}

	if oldHash != newHash {
		return fmt.Errorf("swagger docs are not up to date: run 'mage generate:swagger-docs' and commit the result")
	}
	return nil
}

// YaegiSymbols checks if the yaegi symbol tables in pkg/yaegi_symbols need to be re-generated
func (Check) YaegiSymbols(ctx context.Context) error {
	mg.Deps(initVars)

	// Same hash-compare approach as GotSwag: regenerate in place and compare.
	// The regenerated files are not reset afterwards, which is fine for ci.
	oldHashes := make(map[string]string, len(yaegiSymbolPackages))
	for _, p := range yaegiSymbolPackages {
		hash, err := calculateSha256FileHash(filepath.Join(yaegiSymbolsDir, p.outFile))
		if err != nil {
			return fmt.Errorf("error getting old hash of %s: %w", p.outFile, err)
		}
		oldHashes[p.outFile] = hash
	}

	if err := (Generate{}).YaegiSymbols(ctx); err != nil {
		return err
	}

	var stale []string
	for _, p := range yaegiSymbolPackages {
		hash, err := calculateSha256FileHash(filepath.Join(yaegiSymbolsDir, p.outFile))
		if err != nil {
			return fmt.Errorf("error getting new hash of %s: %w", p.outFile, err)
		}
		if hash != oldHashes[p.outFile] {
			stale = append(stale, p.outFile)
		}
	}

	if len(stale) > 0 {
		return fmt.Errorf("yaegi symbols are not up to date (%s): run 'mage generate:yaegi-symbols' and commit the result", strings.Join(stale, ", "))
	}
	return nil
}

// Translations checks that all translation keys used in the code exist in
// their respective English translation files, and conversely that no unused
// keys exist in that file.
func (Check) Translations() error {
	mg.Deps(initVars)

	var errs []error
	if err := checkAPITranslations(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func checkAPITranslations() error {
	fmt.Println("Checking api translation keys...")

	translationFile := "./pkg/i18n/lang/en.json"
	translations, err := loadTranslations(translationFile)
	if err != nil {
		return fmt.Errorf("error loading api translations: %w", err)
	}

	fmt.Printf("Loaded %d translation keys from %s\n", len(translations), translationFile)

	keys, literals, err := walkAPIForTranslationKeys(".")
	if err != nil {
		return fmt.Errorf("error walking api codebase: %w", err)
	}

	fmt.Printf("Found %d translation key references in the api codebase\n", len(keys))

	// Some api keys are referenced indirectly – e.g. the time.since_* keys are
	// stored as string literals in a struct slice in pkg/utils/humanize_duration.go
	// and looked up via i18n.TP(lang, chunk.key, ...). Any literal that matches a
	// known translation key is treated as a usage hint, so those keys aren't
	// flagged as dead.
	for lit := range literals {
		if translations[lit] {
			keys = append(keys, TranslationKey{Key: lit, FilePath: "<api literal hint>", Line: 0})
		}
	}

	return reportTranslationResults("api", translations, keys, nil)
}

// TranslationKey represents a translation key found in the code
type TranslationKey struct {
	Key      string
	FilePath string
	Line     int
}

// reportTranslationResults checks both missing keys (used in code but not in
// translation file) and dead keys (in translation file but not referenced
// anywhere in code). Returns an error if either kind of mismatch is found.
func reportTranslationResults(label string, translations map[string]bool, keys []TranslationKey, dynamicPrefixes []string) error {
	missingKeys := make(map[string][]TranslationKey)
	usedKeys := make(map[string]bool, len(keys))
	for _, key := range keys {
		usedKeys[key.Key] = true
		if !translations[key.Key] {
			missingKeys[key.Key] = append(missingKeys[key.Key], key)
		}
	}

	// A translation is used if referenced directly, or if its full dotted key
	// starts with any prefix produced by a dynamic call site.
	isCoveredByPrefix := func(k string) bool {
		for _, p := range dynamicPrefixes {
			if strings.HasPrefix(k, p) {
				return true
			}
		}
		return false
	}

	var deadKeys []string
	for k := range translations {
		if usedKeys[k] {
			continue
		}
		if isCoveredByPrefix(k) {
			continue
		}
		deadKeys = append(deadKeys, k)
	}

	var errs []error

	if len(missingKeys) > 0 {
		var missingErrs []error
		for key, occurrences := range missingKeys {
			var keyErrs []error
			for _, occurrence := range occurrences {
				keyErrs = append(keyErrs, fmt.Errorf("- %s:%d", occurrence.FilePath, occurrence.Line))
			}
			missingErrs = append(missingErrs, fmt.Errorf("missing key %s in files:\n%w", key, errors.Join(keyErrs...)))
		}
		errs = append(errs, fmt.Errorf("found %d missing %s translation keys (referenced in code but not in translation file):\n%w", len(missingKeys), label, errors.Join(missingErrs...)))
	}

	if len(deadKeys) > 0 {
		sort.Strings(deadKeys)
		var deadErrs []error
		for _, k := range deadKeys {
			deadErrs = append(deadErrs, fmt.Errorf("- %s", k))
		}
		errs = append(errs, fmt.Errorf("found %d dead %s translation keys (present in translation file but unused in code):\n%w", len(deadKeys), label, errors.Join(deadErrs...)))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	printSuccess(fmt.Sprintf("All %s translation keys are in sync between code and the translation file!", label))
	return nil
}

// loadTranslations loads a translation file and returns a flattened map
func loadTranslations(filePath string) (map[string]bool, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading translation file: %w", err)
	}

	var translationsMap map[string]any
	if err := json.Unmarshal(data, &translationsMap); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	// Flatten the nested structure
	flattenedMap := make(map[string]bool)
	flattenTranslations("", translationsMap, flattenedMap)

	return flattenedMap, nil
}

// flattenTranslations recursively flattens a nested map structure into a flat map with dot-separated keys
func flattenTranslations(prefix string, src map[string]any, dest map[string]bool) {
	for k, v := range src {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch vv := v.(type) {
		case string:
			dest[key] = true
		case map[string]any:
			flattenTranslations(key, vv, dest)
		}
	}
}

// walkAPIForTranslationKeys walks the Go api code and extracts translation
// keys referenced via string-literal arguments to i18n.T / i18n.TP, plus a
// set of dotted string literals as "usage hints" for indirect references
// (e.g. keys stored in a struct slice and passed to i18n.TP via a variable).
func walkAPIForTranslationKeys(rootDir string) ([]TranslationKey, map[string]bool, error) {
	var allKeys []TranslationKey
	allLiterals := make(map[string]bool)

	pkgDir := filepath.Join(rootDir, "pkg")

	err := filepath.Walk(pkgDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}

		if !info.IsDir() && strings.HasSuffix(path, ".go") {
			keys, literals, err := extractAPITranslationKeysFromFile(path)
			if err != nil {
				fmt.Printf("Warning: %v\n", err)
				return nil
			}
			allKeys = append(allKeys, keys...)
			for l := range literals {
				allLiterals[l] = true
			}
		}

		return nil
	})

	return allKeys, allLiterals, err
}

// apiStringLiteralRe matches double-quoted strings in Go source that look like
// dotted translation keys (e.g. "time.since_years"). Used to surface keys that
// are referenced indirectly – for example, stored in a struct field and later
// passed to i18n.TP via a variable.
var apiStringLiteralRe = regexp.MustCompile(`"([a-zA-Z][a-zA-Z0-9_]*(?:\.[a-zA-Z0-9_]+)+)"`)

// extractAPITranslationKeysFromFile extracts all i18n.T/i18n.TP calls with
// a string-literal key, plus all dotted string literals in the file (returned
// as the second value) which are used as usage hints for indirect references.
func extractAPITranslationKeysFromFile(filePath string) ([]TranslationKey, map[string]bool, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	var keys []TranslationKey
	literals := make(map[string]bool)

	// Match i18n.T(ctx, "key") and i18n.TP(ctx, "key", count)
	re := regexp.MustCompile(`i18n\.(T|TP)\([^,]+,\s*"([^"]+)"`)
	matches := re.FindAllSubmatchIndex(content, -1)

	for _, match := range matches {
		if len(match) >= 6 {
			keyStart, keyEnd := match[4], match[5]
			key := string(content[keyStart:keyEnd])

			beforeMatch := content[:keyStart]
			lineCount := bytes.Count(beforeMatch, []byte("\n")) + 1

			keys = append(keys, TranslationKey{
				Key:      key,
				FilePath: filePath,
				Line:     lineCount,
			})
		}
	}

	for _, match := range apiStringLiteralRe.FindAllSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			literals[string(content[match[2]:match[3]])] = true
		}
	}

	return keys, literals, nil
}

func checkGolangCiLintInstalled(ctx context.Context) error {
	mg.Deps(initVars, ensureFrontendDistExists)
	if err := exec.CommandContext(ctx, "golangci-lint").Run(); err != nil && strings.Contains(err.Error(), "executable file not found") {
		return fmt.Errorf("golangci-lint executable failed to run, please manually install golangci-lint by running the command: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.14.0")
	}
	return nil
}

func (Check) Golangci(ctx context.Context) error {
	if err := checkGolangCiLintInstalled(ctx); err != nil {
		return err
	}
	return runAndStreamOutput(ctx, "golangci-lint", "run")
}

func (Check) GolangciFix(ctx context.Context) error {
	if err := checkGolangCiLintInstalled(ctx); err != nil {
		return err
	}
	return runAndStreamOutput(ctx, "golangci-lint", "run", "--fix")
}

// All runs golangci and the swagger test in parallel
func (Check) All() {
	mg.Deps(initVars)
	mg.Deps(
		Check.Golangci,
		Check.GotSwag,
		Check.Translations,
		Check.YaegiSymbols,
	)
}

type Build mg.Namespace

// Clean cleans all build, executable and bindata files
func (Build) Clean(ctx context.Context) error {
	mg.Deps(initVars)
	if err := exec.CommandContext(ctx, "go", "clean", "./...").Run(); err != nil {
		return err
	}
	if err := os.Remove(Executable); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(DIST); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.RemoveAll(BinLocation); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Build builds a vikunja binary, ready to run
func (Build) Build(ctx context.Context) error {
	mg.Deps(initVars, ensureFrontendDistExists)
	return runAndStreamOutput(ctx, "go", "build", goDetectVerboseFlag(), "-tags", Tags, "-ldflags", "-s -w "+Ldflags, "-o", Executable)
}

func (Build) SaveVersionToFile() error {
	// Open the file for writing. If the file doesn't exist, create it.
	// If it exists, truncate it.
	file, err := os.Create("VERSION")
	if err != nil {
		return fmt.Errorf("error creating VERSION file: %w", err)
	}
	defer file.Close()

	// Write the version number to the file
	_, err = file.WriteString(VersionNumber)
	if err != nil {
		return fmt.Errorf("error writing to VERSION file: %w", err)
	}

	fmt.Println("Version number saved successfully to VERSION file")

	return nil
}

type Dev mg.Namespace

// MakeMigration creates a new bare db migration skeleton in pkg/migration.
// If you pass the struct name as an argument, the prompt will be skipped.
func (Dev) MakeMigration(name string) error {
	str := strings.TrimSpace(name)
	if str == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Enter the name of the struct: ")
		s, _ := reader.ReadString('\n')
		str = strings.TrimSpace(s)
	}

	date := time.Now().Format("20060102150405")

	migration := `// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package migration

import (
	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

type ` + str + date + ` struct {
}

func (` + str + date + `) TableName() string {
	return "` + str + `"
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "` + date + `",
		Description: "",
		Migrate: func(tx *xorm.Engine) error {
			return partialSync(tx, ` + str + date + `{})
		},
		Rollback: func(tx *xorm.Engine) error {
			return nil
		},
	})
}
`
	filename := "./pkg/migration/" + date + ".go"
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(migration); err != nil {
		return err
	}

	printSuccess("Migration has been created at %s!", filename)

	return nil
}

// MakeEvent create a new event. Takes the name of the event as the first argument and the module where the event should be created as the second argument. Events will be appended to the pkg/<module>/events.go file.
func (Dev) MakeEvent(name, module string) error {
	name = strcase.ToCamel(name)

	if !strings.HasSuffix(name, "Event") {
		name += "Event"
	}

	eventName := strings.ReplaceAll(strcase.ToDelimited(name, '.'), ".event", "")

	newEventCode := `
// ` + name + ` represents a ` + name + ` event
type ` + name + ` struct {
}

// Name defines the name for ` + name + `
func (t *` + name + `) Name() string {
	return "` + eventName + `"
}
`
	filename := "./pkg/" + module + "/events.go"
	if err := appendToFile(filename, newEventCode); err != nil {
		return err
	}

	printSuccess("The new event has been created successfully! Head over to %s and adjust its content.", filename)

	return nil
}

// MakeListener create a new listener for an event. Takes the name of the listener, the name of the event to listen to and the module where everything should be placed as parameters.
func (Dev) MakeListener(name, event, module string) error {
	name = strcase.ToCamel(name)
	listenerName := strcase.ToDelimited(name, '.')
	listenerCode := `
// ` + name + `  represents a listener
type ` + name + ` struct {
}

// Name defines the name for the ` + name + ` listener
func (s *` + name + `) Name() string {
	return "` + listenerName + `"
}

// Handle is executed when the event ` + name + ` listens on is fired
func (s *` + name + `) Handle(msg *message.Message) (err error) {
	event := &` + event + `{}
	err = json.Unmarshal(msg.Payload, event)
	if err != nil {
		return err
	}

	return nil
}
`
	filename := "./pkg/" + module + "/listeners.go"

	//////
	// Register the listener

	file, err := os.Open(filename)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(file)
	var idx int64
	for scanner.Scan() {
		if scanner.Text() == "}" {
			// idx -= int64(len(scanner.Text()))
			break
		}
		idx += int64(len(scanner.Bytes()) + 1)
	}
	file.Close()

	registerListenerCode := `	events.RegisterListener((&` + event + `{}).Name(), &` + name + `{})
`

	f, err := os.OpenFile(filename, os.O_RDWR, 0o600)
	if err != nil {
		return err
	}

	defer f.Close()

	if _, err := f.Seek(idx, 0); err != nil {
		return err
	}
	remainder, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	if _, err := f.Seek(idx, 0); err != nil {
		return err
	}
	if _, err := f.Write([]byte(registerListenerCode)); err != nil {
		return err
	}
	if _, err := f.Write(remainder); err != nil {
		return err
	}

	///////
	// Append the listener code
	if err := appendToFile(filename, listenerCode); err != nil {
		return err
	}

	printSuccess("The new listener has been created successfully! Head over to %s and adjust its content.", filename)

	return nil
}

// MakeNotification create a new notification. Takes the name of the notification as the first argument and the module where the notification should be created as the second argument. Notifications will be appended to the pkg/<module>/notifications.go file.
func (Dev) MakeNotification(name, module string) error {
	name = strcase.ToCamel(name)

	if !strings.HasSuffix(name, "Notification") {
		name += "Notification"
	}

	notficationName := strings.ReplaceAll(strcase.ToDelimited(name, '.'), ".notification", "")

	newNotificationCode := `
// ` + name + ` represents a ` + name + ` notification
type ` + name + ` struct {
}

// ToMail returns the mail notification for ` + name + `
func (n *` + name + `) ToMail(lang string) *notifications.Mail {
	return notifications.NewMail().
		Subject("").
		Greeting("Hi ").
		Line("").
		Action("", "")
}

// ToDB returns the ` + name + ` notification in a format which can be saved in the db
func (n *` + name + `) ToDB() any {
	return nil
}

// Name returns the name of the notification
func (n *` + name + `) Name() string {
	return "` + notficationName + `"
}

`
	filename := "./pkg/" + module + "/notifications.go"
	if err := appendToFile(filename, newNotificationCode); err != nil {
		return err
	}

	printSuccess("The new notification has been created successfully! Head over to %s and adjust its content.", filename)

	return nil
}

type Generate mg.Namespace

const DefaultConfigYAMLSamplePath = "config.yml.sample"

// SwaggerDocs generates the swagger docs from the code annotations
func (Generate) SwaggerDocs(ctx context.Context) error {
	mg.Deps(initVars)

	if err := checkAndInstallGoTool(ctx, "swag", "github.com/swaggo/swag/cmd/swag"); err != nil {
		return err
	}
	return runAndStreamOutput(ctx, "swag", "init", "-g", "./pkg/routes/routes.go", "--parseDependency", "-d", ".", "-o", "./pkg/swagger")
}

const yaegiSymbolsDir = "./pkg/yaegi_symbols"

// yaegiSymbolPackages maps extracted import paths to their checked-in file names in pkg/yaegi_symbols.
var yaegiSymbolPackages = []struct {
	importPath string
	outFile    string
}{
	{"code.vikunja.io/api/pkg/config", "vikunja_config.go"},
	{"code.vikunja.io/api/pkg/db", "vikunja_db.go"},
	{"code.vikunja.io/api/pkg/events", "vikunja_events.go"},
	{"code.vikunja.io/api/pkg/log", "vikunja_log.go"},
	{"code.vikunja.io/api/pkg/models", "vikunja_models.go"},
	{"code.vikunja.io/api/pkg/plugins", "vikunja_plugins.go"},
	{"code.vikunja.io/api/pkg/user", "vikunja_user.go"},
	{"github.com/labstack/echo/v5", "echo.go"},
	{"github.com/ThreeDotsLabs/watermill/message", "watermill.go"},
	{"github.com/spf13/viper", "viper.go"},
	{"src.techknowlogick.com/xormigrate", "xormigrate.go"},
	{"xorm.io/xorm", "xorm.go"},
}

// YaegiSymbols regenerates the yaegi symbol tables in pkg/yaegi_symbols so
// interpreted plugins see the current exported API.
func (Generate) YaegiSymbols(ctx context.Context) error {
	mg.Deps(initVars)

	// Same replacer yaegi's extract command uses to derive the output file name.
	replacer := strings.NewReplacer("/", "-", ".", "_", "~", "_")

	for _, p := range yaegiSymbolPackages {
		c := exec.CommandContext(ctx, "go", "run", "github.com/traefik/yaegi/cmd/yaegi", "extract", p.importPath) //nolint:gosec // import paths come from the hard-coded yaegiSymbolPackages list
		c.Dir = yaegiSymbolsDir
		c.Env = append(os.Environ(), "GOPACKAGE=yaegi_symbols")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		fmt.Printf("%s\n\n", c.String())
		if err := c.Run(); err != nil {
			return fmt.Errorf("error extracting yaegi symbols for %s: %w", p.importPath, err)
		}

		// yaegi extract exits 0 even when extraction fails, so the missing
		// output file surfaces here as a rename error.
		generated := filepath.Join(yaegiSymbolsDir, replacer.Replace(p.importPath)+".go")
		if err := os.Rename(generated, filepath.Join(yaegiSymbolsDir, p.outFile)); err != nil {
			return fmt.Errorf("error renaming extracted yaegi symbols for %s: %w", p.importPath, err)
		}
	}
	return nil
}

type ConfigNode struct {
	Key      string        `json:"key,omitempty"`
	Value    any           `json:"default_value,omitempty"`
	Comment  string        `json:"comment,omitempty"`
	Children []*ConfigNode `json:"children,omitempty"`
}

func convertConfigJSONToYAML(node *ConfigNode, indent int, isTopLevel bool, parentKey string, commentOut bool) string {
	var result strings.Builder

	writeComment := func(comment string, indent int) {
		indent = int(math.Max(float64(indent), 0))
		if comment != "" {
			commentLines := strings.Split(comment, "\n")
			for _, line := range commentLines {
				result.WriteString(strings.Repeat("  ", indent))
				result.WriteString("# " + line + "\n")
			}
		}
	}

	writeLine := func(line string, indent int) {
		indent = int(math.Max(float64(indent), 0))
		if commentOut {
			result.WriteString(strings.Repeat("  ", indent) + "# " + line + "\n")
		} else {
			result.WriteString(strings.Repeat("  ", indent) + line + "\n")
		}
	}

	if isTopLevel {
		writeComment(node.Comment, indent)
	}

	if node.Key != "" {
		if !isTopLevel {
			writeComment(node.Comment, indent)
		}
		line := node.Key + ":"
		if node.Value != nil {
			value := node.Value
			if value == nil {
				value = node.Value
			}
			line += " " + formatValue(value)
		}
		writeLine(line, indent)
	}

	if len(node.Children) > 0 {
		isProviders := node.Key == "providers" && parentKey == "openid"
		isArray := len(node.Children) > 0 && node.Children[0].Key == ""
		for i, child := range node.Children {
			switch {
			case isProviders:
				writeComment(child.Comment, indent+1)
				writeLine("-", indent+1)
				result.WriteString(convertConfigJSONToYAML(child, indent+1, false, node.Key, commentOut))
			case isArray:
				writeComment(child.Comment, indent+1)
				writeLine("- "+formatValue(child.Value), indent+1)
			default:
				result.WriteString(convertConfigJSONToYAML(child, indent+1, false, node.Key, commentOut))
			}
			if i == len(node.Children)-1 && !isProviders && !isArray {
				writeLine("", indent)
			}
		}
	}

	return result.String()
}

func formatValue(value any) string {
	switch v := value.(type) {
	case string:
		if intValue, err := strconv.Atoi(v); err == nil {
			return fmt.Sprintf("%d", intValue)
		}
		if floatValue, err := strconv.ParseFloat(v, 64); err == nil {
			return fmt.Sprintf("%g", floatValue)
		}
		if boolValue, err := strconv.ParseBool(v); err == nil {
			return fmt.Sprintf("%v", boolValue)
		}
		return fmt.Sprintf("%q", v)
	case float64:
		return fmt.Sprintf("%g", v)
	case int:
		return fmt.Sprintf("%d", v)
	case bool:
		return fmt.Sprintf("%v", v)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

func generateConfigYAMLFromJSON(yamlPath string, commented bool) {
	jsonData, err := os.ReadFile("config-raw.json")
	if err != nil {
		fmt.Println("Error reading JSON file:", err)
		return
	}

	var root ConfigNode
	err = json.Unmarshal(jsonData, &root)
	if err != nil {
		fmt.Println("Error parsing JSON:", err)
		return
	}

	yamlData := convertConfigJSONToYAML(&root, -1, true, "", commented)

	err = os.WriteFile(yamlPath, []byte(yamlData), 0o600)
	if err != nil {
		fmt.Println("Error writing YAML file:", err)
		return
	}

	fmt.Println("Successfully generated " + yamlPath)
}

// ConfigYAML create a yaml config file from the config-raw.json definition
func (Generate) ConfigYAML(commented bool) {
	generateConfigYAMLFromJSON(DefaultConfigYAMLSamplePath, commented)
}

// ScalarBundle downloads the Scalar API reference standalone JS bundle into
// pkg/routes/api/v2/scalar/. Version is pinned to match the Scalar version
// used in Huma's internal docs at the time of last update.
func (Generate) ScalarBundle() error {
	const (
		version = "1.44.20"
		dest    = "pkg/routes/api/v2/scalar/scalar.standalone.js"
	)
	url := fmt.Sprintf("https://unpkg.com/@scalar/api-reference@%s/dist/browser/standalone.js", version)

	fmt.Printf("Downloading Scalar bundle %s from %s\n", version, url)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //nolint:gosec // This is a dev-only mage task and the URL is hard-coded above.
	if err != nil {
		return fmt.Errorf("build scalar bundle request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download scalar bundle: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download scalar bundle: unexpected status %s", resp.Status)
	}

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return fmt.Errorf("read scalar bundle body: %w", err)
	}

	if err := os.WriteFile(dest, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}

	fmt.Printf("Wrote %d bytes to %s\n", buf.Len(), dest)
	return nil
}

func localBranchExists(ctx context.Context, name string) bool {
	return exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+name).Run() == nil //nolint:gosec // This is a dev-only mage task and the branch name is supplied by the developer running it.
}

func remoteBranchExists(ctx context.Context, name string) bool {
	return exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/remotes/"+name).Run() == nil //nolint:gosec // This is a dev-only mage task and the branch name is supplied by the developer running it.
}

// PrepareWorktree creates a new git worktree for development.
// The first argument is the name, which becomes both the folder name and branch name.
// If a local branch with that name exists, it is checked out.
// If a remote branch origin/<name> exists, a local tracking branch is created.
// Otherwise a new branch is created.
// The second argument is a path to a plan file that will be moved to the new worktree (pass "" to skip).
// The worktree is created in the parent directory (../).
// It also copies the current config.yml with an updated rootpath, and initializes the frontend.
func (Dev) PrepareWorktree(ctx context.Context, name string, planPath string) error {
	if name == "" {
		return fmt.Errorf("name is required: mage dev:prepare-worktree <name> <plan-path>")
	}

	// Get the parent directory path
	worktreePath := filepath.Join("..", name)

	fmt.Printf("Creating worktree at %s with branch %s...\n", worktreePath, name)

	// Refresh remote refs so remote-branch detection is reliable.
	fetchCmd := exec.CommandContext(ctx, "git", "fetch", "origin")
	fetchCmd.Stdout = os.Stdout
	fetchCmd.Stderr = os.Stderr
	if err := fetchCmd.Run(); err != nil {
		fmt.Printf("Warning: git fetch failed: %v\n", err)
	}

	var worktreeArgs []string
	switch {
	case localBranchExists(ctx, name):
		fmt.Printf("Local branch %s exists, checking it out.\n", name)
		worktreeArgs = []string{"worktree", "add", worktreePath, name}
	case remoteBranchExists(ctx, "origin/"+name):
		fmt.Printf("Remote branch origin/%s exists, creating tracking branch.\n", name)
		worktreeArgs = []string{"worktree", "add", "--track", "-b", name, worktreePath, "origin/" + name}
	default:
		fmt.Printf("Creating new branch %s.\n", name)
		worktreeArgs = []string{"worktree", "add", worktreePath, "-b", name}
	}

	cmd := exec.CommandContext(ctx, "git", worktreeArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}
	printSuccess("Worktree created successfully!")

	// Copy and modify config.yml
	configSrc := "config.yml"
	configDst := filepath.Join(worktreePath, "config.yml")

	if _, err := os.Stat(configSrc); err == nil {
		configContent, err := os.ReadFile(configSrc)
		if err != nil {
			return fmt.Errorf("failed to read config.yml: %w", err)
		}

		// Replace the rootpath value
		re := regexp.MustCompile(`(?m)^(\s*rootpath:\s*)"[^"]*"`)
		newConfig := re.ReplaceAllString(string(configContent), `${1}"`+worktreePath+`"`)

		// Also handle unquoted rootpath values
		re2 := regexp.MustCompile(`(?m)^(\s*rootpath:\s*)(/[^\s\n]+)`)
		newConfig = re2.ReplaceAllString(newConfig, `${1}"`+worktreePath+`"`)

		if err := os.WriteFile(configDst, []byte(newConfig), 0o600); err != nil {
			return fmt.Errorf("failed to write config.yml: %w", err)
		}
		printSuccess("Config copied with updated rootpath!")
	} else {
		fmt.Println("Warning: config.yml not found, skipping config copy")
	}

	// Copy .claude/settings.local.json if it exists
	claudeSettingsSrc := filepath.Join(".claude", "settings.local.json")
	if _, err := os.Stat(claudeSettingsSrc); err == nil {
		claudeDir := filepath.Join(worktreePath, ".claude")
		if err := os.MkdirAll(claudeDir, 0o755); err != nil {
			return fmt.Errorf("failed to create .claude directory: %w", err)
		}
		claudeSettingsDst := filepath.Join(claudeDir, "settings.local.json")
		if err := copyFile(claudeSettingsSrc, claudeSettingsDst); err != nil {
			return fmt.Errorf("failed to copy .claude/settings.local.json: %w", err)
		}
		printSuccess("Claude settings copied!")
	}

	// Copy plan file if provided
	if planPath != "" {
		planPath = strings.TrimSpace(planPath)
		if planPath != "" {
			// Create plans directory in the new worktree
			plansDir := filepath.Join(worktreePath, "plans")
			if err := os.MkdirAll(plansDir, 0o755); err != nil {
				return fmt.Errorf("failed to create plans directory: %w", err)
			}

			// Determine source path (relative to current directory or absolute)
			srcPlanPath := planPath
			if !filepath.IsAbs(planPath) {
				srcPlanPath = planPath
			}

			if _, err := os.Stat(srcPlanPath); err != nil {
				return fmt.Errorf("plan file not found: %s", srcPlanPath)
			}

			dstPlanPath := filepath.Join(plansDir, filepath.Base(planPath))
			if err := os.Rename(srcPlanPath, dstPlanPath); err != nil {
				return fmt.Errorf("failed to move plan file: %w", err)
			}
			printSuccess("Plan file moved to %s!", dstPlanPath)
		}
	}

	// Initialize the web frontend
	fmt.Println("Initializing web frontend...")

	if err := ensureFrontendDistExistsIn(worktreePath); err != nil {
		return err
	}

	npmCmd := exec.CommandContext(ctx, "npm", "ci")
	npmCmd.Dir = filepath.Join(worktreePath, "web")
	npmCmd.Stdout = os.Stdout
	npmCmd.Stderr = os.Stderr
	if err := npmCmd.Run(); err != nil {
		return fmt.Errorf("failed to run npm ci: %w", err)
	}

	printSuccess("Web frontend initialized!")
	printSuccess("\nWorktree ready at: %s", worktreePath)
	printSuccess("Branch: %s", name)
	fmt.Println("\nTo start working:")
	fmt.Printf("  cd %s\n", worktreePath)

	return nil
}

type Plugins mg.Namespace

// Build compiles a Go plugin at the provided path.
func (Plugins) Build(ctx context.Context, pathToSourceFiles string) error {
	mg.Deps(initVars)
	if pathToSourceFiles == "" {
		return fmt.Errorf("please provide a plugin path")
	}

	// Convert relative path to absolute path
	if !strings.HasPrefix(pathToSourceFiles, "/") {
		absPath, err := filepath.Abs(pathToSourceFiles)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		pathToSourceFiles = absPath
	}

	out := filepath.Join("plugins", filepath.Base(pathToSourceFiles)+".so")
	return runAndStreamOutput(ctx, "go", "build", "-buildmode=plugin", "-tags", Tags, "-o", out, pathToSourceFiles)
}
