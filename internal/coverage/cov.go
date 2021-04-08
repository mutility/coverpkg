package coverage

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mutility/coverpkg/internal/diag"
)

var ErrNoPackages = errors.New("no packages specified")

type (
	// StatementData records all statements (including location data) and covered status
	StatementData map[stmt]bool // StatementData skips EachPath as EachStatement is not unique per file.

	StmtCount   struct{ Count, Covered int }
	FileData    map[string]StmtCount
	PackageData map[string]StmtCount
	ModuleData  map[string]StmtCount

	StmtDelta   struct{ BaseCount, BaseCovered, HeadCount, HeadCovered int }
	FileDelta   map[string]StmtDelta
	PathDelta   map[string]StmtDelta
	ModuleDelta map[string]StmtCount
)

func (sd StatementData) EachStatement(fn func(path, pos string, count int, covered int)) {
	for k, v := range sd {
		path, pos := k.loc()
		fn(path, pos, k.count, k.covered(v))
	}
}

func (sd StatementData) EachFile(fn func(path string, count int, covered int)) {
	for k, v := range sd {
		fn(k.file(), k.count, k.covered(v))
	}
}

func (sd StatementData) EachPackage(fn func(path string, count int, covered int)) {
	for k, v := range sd {
		fn(k.pkg(), k.count, k.covered(v))
	}
}

func (sd StatementData) EachModule(fn func(path string, count int, covered int)) {
	for k, v := range sd {
		fn(k.mod(), k.count, k.covered(v))
	}
}

func (fd FileData) EachPath(fn func(path string, count int, covered int)) { fd.EachFile(fn) }

func (fd FileData) EachFile(fn func(path string, count int, covered int)) {
	for k, v := range fd {
		fn(k, v.Count, v.Covered)
	}
}

func (fd FileData) EachPackage(fn func(path string, count int, covered int)) {
	for k, v := range fd {
		fn(pathpkg(nil, k), v.Count, v.Covered)
	}
}

func (fd FileData) EachModule(fn func(path string, count int, covered int)) {
	for k, v := range fd {
		fn(pathmod(nil, k), v.Count, v.Covered)
	}
}

func nth(s string, r rune, n int) int {
	c := 0
	for i, si := range s {
		if si == r {
			c++
			if c == n {
				return i
			}
		}
	}
	return -1
}

func (fd FileData) Paths() []string {
	files := make([]string, 0, len(fd))
	for p := range fd {
		files = append(files, string(p))
	}
	sort.Strings(files)
	return files
}

func (fd FileData) Detail(pkg string) Counts {
	c := fd[pkg]
	return Counts{Covered: c.Covered, Total: c.Count}
}

func (pd PackageData) EachPath(fn func(path string, count int, covered int)) { pd.EachPackage(fn) }

func (pd PackageData) EachPackage(fn func(path string, count int, covered int)) {
	for k, v := range pd {
		fn(k, v.Count, v.Covered)
	}
}

func (pd PackageData) EachModule(fn func(path string, count int, covered int)) {
	for k, v := range pd {
		fn(pathmod(nil, k), v.Count, v.Covered)
	}
}

func Diff(log diag.Interface, old, new EachPather) PathDelta {
	var oldEach, newEach func(func(string, int, int))
	oldPD, _ := old.(EachPackager)
	newPD, _ := new.(EachPackager)
	oldFD, _ := old.(EachFiler)
	newFD, _ := new.(EachFiler)

	if oldFD != nil && newFD != nil {
		oldEach = oldFD.EachFile
		newEach = newFD.EachFile
	} else if oldPD != nil && newPD != nil {
		oldEach = oldPD.EachPackage
		newEach = newPD.EachPackage
		if oldFD != nil {
			oldEach = ByPackage(log, oldFD).EachPackage
		}
		if newFD != nil {
			newEach = ByPackage(log, newFD).EachPackage
		}
	} else {
		oldEach = old.EachPath
		newEach = new.EachPath
		if oldPD != nil {
			oldEach = ByModule(log, oldPD).EachModule
		}
		if newPD != nil {
			newEach = ByModule(log, newPD).EachModule
		}
	}

	delta := make(PathDelta)
	oldEach(func(path string, count, covered int) {
		cc := delta[path]
		cc.BaseCount = count
		cc.BaseCovered += covered
		delta[path] = cc
	})
	newEach(func(path string, count int, covered int) {
		cc := delta[path]
		cc.HeadCount = count
		cc.HeadCovered += covered
		delta[path] = cc
	})
	return delta
}

func (pd PackageData) Paths() []string {
	pkgs := make([]string, 0, len(pd))
	for p := range pd {
		pkgs = append(pkgs, string(p))
	}
	sort.Strings(pkgs)
	return pkgs
}

func (pd PackageData) Detail(pkg string) Counts {
	c := pd[pkg]
	return Counts{Covered: c.Covered, Total: c.Count, IsAggregate: true}
}

func (md ModuleData) EachPath(fn func(path string, count int, covered int)) { md.EachModule(fn) }

func (md ModuleData) EachModule(fn func(path string, count int, covered int)) {
	for k, v := range md {
		fn(k, v.Count, v.Covered)
	}
}

func (md ModuleData) Paths() []string {
	pkgs := make([]string, 0, len(md))
	for p := range md {
		pkgs = append(pkgs, string(p))
	}
	sort.Strings(pkgs)
	return pkgs
}

func (md ModuleData) Detail(pkg string) Counts {
	c := md[pkg]
	return Counts{Covered: c.Covered, Total: c.Count, IsAggregate: true}
}

func (pd PathDelta) Paths() []string {
	pkgs := make([]string, 0, len(pd))
	for p := range pd {
		pkgs = append(pkgs, string(p))
	}
	sort.Strings(pkgs)
	return pkgs
}

func (pd PathDelta) Detail(pkg string) Counts {
	c := pd[pkg]
	return Counts{Covered: c.HeadCovered, Total: c.HeadCount, IsAggregate: pkg != "."}
}

func (pd PathDelta) BaseDetail(pkg string) Counts {
	c := pd[pkg]
	return Counts{Covered: c.BaseCovered, Total: c.BaseCount, IsAggregate: pkg != "."}
}

func CollectStatements(ctx diag.Context, options *TestOptions) (StatementData, error) {
	prof, err := coverprofile(ctx, options)
	if err != nil {
		return nil, err
	}
	return LoadProfile(ctx, prof, options)
}

func CollectFiles(ctx diag.Context, options *TestOptions) (FileData, error) {
	stmts, err := CollectStatements(ctx, options)
	if err != nil {
		return nil, err
	}

	return ByFiles(ctx, stmts), nil
}

func Percent(c EachPather) float64 {
	totalct, totalcov := 0, 0
	c.EachPath(func(_ string, count, covered int) {
		totalct += count
		totalcov += covered
	})

	if totalct == 0 {
		return 0.0
	}
	return float64(totalcov*100) / float64(totalct)
}

type (
	EachStatementer interface {
		// EachStatement calls back once for each statement.
		EachStatement(func(path, pos string, count int, covered int))
	}

	EachFiler interface {
		// EachFile calls back at least once for each file.
		// Counts and coverage for the file should be summed.
		EachFile(func(path string, count int, covered int))
	}

	EachPackager interface {
		// EachPackage calls back at least once for each package.
		// Counts and coverage for the package should be summed.
		EachPackage(func(path string, count int, covered int))
	}

	EachModuler interface {
		// EachModuler calls back at least once for each module.
		// Counts and coverage for the package should be summed.
		EachModule(func(path string, count int, covered int))
	}

	EachPather interface {
		// EachPath calls back exactly once for each tracked path.
		// Unlike the other Each* methods, this is at the 'native' granularity
		// for the underlying storage.
		EachPath(func(path string, count int, covered int))
	}
)

func ByFiles(log diag.Interface, stmts EachStatementer) FileData {
	_ = log
	fd := make(FileData)
	stmts.EachStatement(func(path, _ string, count int, covered int) {
		cc := fd[path]
		cc.Count += count
		cc.Covered += covered
		fd[path] = cc
	})
	return fd
}

func ByPackage(log diag.Interface, files EachFiler) PackageData {
	pd := make(PackageData)
	files.EachFile(func(path string, count int, covered int) {
		rsl := strings.LastIndexByte(path, '/')
		if rsl < 0 {
			diag.Debug(log, "can't find file in:", path)
			return
		}

		ploc := path[:rsl]
		cc := pd[ploc]
		cc.Count += count
		cc.Covered += covered
		pd[ploc] = cc
	})
	return pd
}

func ByRoot(log diag.Interface, pkgs EachPackager) PackageData {
	pd := make(PackageData)
	pkgs.EachPackage(func(path string, count int, covered int) {
		root := pathroot(log, path)
		cc := pd[root]
		cc.Count += count
		cc.Covered += covered
		pd[root] = cc
	})
	return pd
}

func pathroot(log diag.Interface, path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 4 {
		parts = parts[:4]
	}
	if len(parts) < 2 && len(path) > 0 {
		diag.Debug(log, "can't find root in:", path)
	}
	return strings.Join(parts, "/")
}

func pathpkg(log diag.Interface, path string) string {
	n := strings.LastIndexByte(path, '/')
	if n < 0 {
		diag.Debug(log, "cant' find package in:", path)
		return path
	}
	return path[:n]
}

func pathmod(log diag.Interface, path string) string {
	n := nth(path, '/', 2)
	if n < 0 {
		diag.Debug(log, "can't find module in:", path)
		return path
	}
	return path[:n]
}

func ByModule(log diag.Interface, pkgs EachPackager) ModuleData {
	md := make(ModuleData)
	pkgs.EachPackage(func(path string, count int, covered int) {
		parts := strings.Split(path, "/")
		if len(parts) > 3 {
			parts = parts[:3]
		}
		if len(parts) < 2 && len(path) > 0 {
			diag.Debug(log, "can't find module in:", path)
		}
		mod := strings.Join(parts, "/")

		cc := md[mod]
		cc.Count += count
		cc.Covered += covered
		md[mod] = cc
	})
	return md
}

type TestOptions struct {
	Flags    []string
	Packages []string
	Excludes []string
}

func (o *TestOptions) excludes(path string) bool {
	for _, ex := range o.Excludes {
		if strings.HasPrefix(path, ex+"/") ||
			strings.Contains(path, "/"+ex+"/") {
			return true
		}
	}
	return false
}

var DefaultTestOptions = &TestOptions{
	Flags:    nil,
	Packages: []string{"."},
	Excludes: nil,
}

// coverprofile collects a coverprofile and returns the filename
func coverprofile(log diag.Interface, options *TestOptions) (string, error) {
	prof, err := os.CreateTemp("", "covpkg*")
	if err != nil {
		return "", err
	}
	prof.Close()

	if options == nil {
		options = DefaultTestOptions
	}
	if len(options.Packages) == 0 {
		options.Packages = append(options.Packages, ".")
	}
	diag.Debug(log, "Creating profile in:", prof.Name(), "packages", options.Packages)

	pkgs := make([]string, len(options.Packages))
	for i, arg := range options.Packages {
		if st, err := os.Stat(arg); err == nil && st.IsDir() {
			if rel, err := filepath.Rel(".", arg); err == nil {
				if rel == "." {
					pkgs[i] = "./..."
				} else {
					pkgs[i] = "./" + rel + "/..."
				}
				continue
			}
		}
		pkgs[i] = arg
	}

	diag.Debug(log, "run> go test -coverprofile", prof.Name(), "-coverpkg", strings.Join(pkgs, ","), strings.Join(pkgs, " "))
	cmd := exec.Command("go", append([]string{"test", "-coverprofile", prof.Name(), "-coverpkg", strings.Join(pkgs, ",")}, pkgs...)...)
	err = cmd.Run()
	if err != nil {
		os.Remove(prof.Name())
		return "", fmt.Errorf("tests failed: %w", err)
	}
	return prof.Name(), nil
}

type stmt struct {
	filepos string
	count   int
}

func (s stmt) loc() (path, pos string) {
	n := strings.LastIndexByte(s.filepos, ':')
	return s.filepos[:n], s.filepos[n+1:]
}

func (s stmt) file() string {
	n := strings.LastIndexByte(s.filepos, ':')
	return s.filepos[:n]
}

func (s stmt) pkg() string {
	n := strings.LastIndexByte(s.filepos, ':')
	n = strings.LastIndexByte(s.filepos[:n], '/')
	return s.filepos[:n]
}

func (s stmt) mod() string {
	return pathmod(nil, s.filepos)
}

func (s stmt) covered(cov bool) int {
	if cov {
		return s.count
	}
	return 0
}

// LoadProfile loads statement coverage from a coverprofile file.
func LoadProfile(ctx diag.Context, prof string, options *TestOptions) (StatementData, error) {
	r, err := os.Open(prof)
	if err != nil {
		return nil, err
	}

	stmts, err := readStatements(ctx, r, options)

	if err := r.Close(); err != nil {
		diag.Debug(ctx, "closing coverprofile:", err)
	}
	return stmts, err
}

func readStatements(ctx diag.Context, r io.Reader, options *TestOptions) (StatementData, error) {
	scan := bufio.NewScanner(r)
	return scanStatements(ctx, scan, options)
}

func scanStatements(ctx diag.Context, s *bufio.Scanner, options *TestOptions) (StatementData, error) {
	stmts := make(StatementData)

	for s.Scan() && ctx.Err() == nil {
		line := s.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}

		f := strings.Fields(line)
		if len(f) != 3 {
			diag.Debug(ctx, "invalid line:", line)
			continue
		}

		if options.excludes(f[0]) {
			continue
		}

		ct, err := strconv.Atoi(f[1])
		if err != nil {
			diag.Debug(ctx, "invalid fields:", line)
			return nil, err
		}
		loc := stmt{f[0], ct}
		cov := f[2] != "0"
		stmts[loc] = cov || stmts[loc]
	}

	return stmts, ctx.Err()
}

type module string

func Module(ctx diag.Context) module {
	diag.Debug(ctx, "exec> go list -f {{ .Module }}")
	cmd := exec.CommandContext(ctx, "go", "list", "-f", "{{ .Module }}")
	mod, err := cmd.Output()
	if err != nil {
		return ""
	}
	return module(bytes.TrimSpace(mod))
}