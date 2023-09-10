package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mutility/diag"
)

type GitHubAction struct {
	w io.Writer
}

var ghaEscaper = strings.NewReplacer("%", "%25", "\n", "%0A", "\r", "%0D")

// At specifies the location of an error or warning.
// Use like gha.At(filename [, line, col]).Error(information...)
// File is required (or skip use of At); line and col are optional.
func (gha *GitHubAction) At(file string, linecol ...int) *ghaPos {
	if len(linecol) > 1 {
		return &ghaPos{gha.w, file, linecol[0], linecol[1]}
	} else if len(linecol) > 0 {
		return &ghaPos{gha.w, file, linecol[0], 0}
	}
	return &ghaPos{w: gha.w, file: file}
}

// Debug emits a debug message. GitHub only shows these if you've set secret:
//
//	ACTIONS_STEP_DEBUG=true
func (gha *GitHubAction) Debug(a ...interface{}) {
	fmt.Fprintf(gha.w, "::debug::%s\n", sprint(a))
}

// Debugf emits a debug message. GitHub only shows these if you've set secret:
//
//	ACTIONS_STEP_DEBUG=true
func (gha *GitHubAction) Debugf(format string, a ...interface{}) {
	fmt.Fprintf(gha.w, "::debug::%s\n", sprintf(format, a))
}

// Print emits regular output.
func (gha *GitHubAction) Print(a ...interface{}) {
	fmt.Fprintf(gha.w, "%s\n", sprint(a))
}

// Printf emits regular output.
func (gha *GitHubAction) Printf(format string, a ...interface{}) {
	fmt.Fprintf(gha.w, "%s\n", sprintf(format, a))
}

// Error emits an error message.
func (gha *GitHubAction) Error(a ...interface{}) {
	fmt.Fprintf(gha.w, "::error::%s\n", sprint(a))
}

// Errorf emits an error message.
func (gha *GitHubAction) Errorf(format string, a ...interface{}) {
	fmt.Fprintf(gha.w, "::error::%s\n", sprintf(format, a))
}

// ErrorAt emits an error message at a position. Set line and column to 0 to omit.
func (gha *GitHubAction) ErrorAt(file string, line, col int, a ...interface{}) {
	(&ghaPos{gha.w, file, line, col}).Error(a...)
}

// ErrorAtf emits an error message at a position. Set line and column to 0 to omit.
func (gha *GitHubAction) ErrorAtf(file string, line, col int, format string, a ...interface{}) {
	(&ghaPos{gha.w, file, line, col}).Errorf(format, a...)
}

// Warning emits an warning message.
func (gha *GitHubAction) Warning(a ...interface{}) {
	fmt.Fprintf(gha.w, "::warning::%s\n", sprint(a))
}

// Warningf emits an warning message.
func (gha *GitHubAction) Warningf(format string, a ...interface{}) {
	fmt.Fprintf(gha.w, "::warning::%s\n", sprintf(format, a))
}

// WarningAt emits an warning message at a position. Set line and column to 0 to omit.
func (gha *GitHubAction) WarningAt(file string, line, col int, a ...interface{}) {
	(&ghaPos{gha.w, file, line, col}).Warning(a...)
}

// WarningAtf emits an warning message at a position. Set line and column to 0 to omit.
func (gha *GitHubAction) WarningAtf(file string, line, col int, format string, a ...interface{}) {
	(&ghaPos{gha.w, file, line, col}).Warningf(format, a...)
}

// Group groups output in a GitHub actions log
func (gha *GitHubAction) Group(title string, fn func(diag.Interface)) {
	fmt.Fprintf(gha.w, "::group::%s\n", ghaUnsafe(title))
	fn(gha)
	fmt.Fprint(gha.w, "::endgroup::\n")
}

// MaskValue requests the GitHub actions log mask this value
func (gha *GitHubAction) MaskValue(secret string) {
	fmt.Fprintf(gha.w, "::add-mask::%s\n", ghaUnsafe(secret))
}

// SetOutput sets an output to the provided value.
func (gha *GitHubAction) SetOutput(name, value string) {
	_, err := appendFilef(cfg.SetOutput, "%s=%s\n", name, ghaUnsafe(value))
	switch err {
	case nil:
		return
	case errEmptyPath:
		gha.Error("GITHUB_OUTPUT not available")
	default:
		gha.Error(err)
	}
}

// SetEnv sets an environment variable for future actions.
func (gha *GitHubAction) SetEnv(name, value string) {
	format := "%s=%s\n"
	if strings.ContainsRune(value, '\n') {
		format = "%s=<<END_%[1]s\n%s\nEND_%[1]s\n"
	}
	_, err := appendFilef(cfg.SetEnv, format, name, value)
	switch err {
	case nil:
		return
	case errEmptyPath:
		gha.Error("GITHUB_ENV not available")
	default:
		gha.Error(err)
	}
}

// AddPath sets a path for future actions.
func (gha *GitHubAction) AddPath(path string) {
	_, err := appendFilef(cfg.SetPath, "%s\n", path)
	switch err {
	case nil:
		return
	case errEmptyPath:
		gha.Error("GITHUB_PATH not available")
	default:
		gha.Error(err)
	}
}

func (gha *GitHubAction) Event(path string) *GitHubEvent {
	f, err := os.Open(path)
	if err != nil {
		gha.Error("opening event data:", err)
		return nil
	}
	d := json.NewDecoder(f)
	ghe := GitHubEvent{}
	if err = d.Decode(&ghe); err != nil {
		gha.Error("decoding event data:", err)
	}
	f.Close()
	return &ghe
}

// ghaPos reports a file location for Error and Warning messages.
type ghaPos struct {
	w    io.Writer
	file string
	line int
	col  int
}

// Error emits an error message.
func (pos *ghaPos) Error(a ...interface{}) {
	fmt.Fprintf(pos.w, "::error%v::%s\n", pos, sprint(a))
}

// Errorf emits an error message.
func (pos *ghaPos) Errorf(format string, a ...interface{}) {
	fmt.Fprintf(pos.w, "::error%v::%s\n", pos, sprintf(format, a))
}

// Warning emits an warning message.
func (pos *ghaPos) Warning(a ...interface{}) {
	fmt.Fprintf(pos.w, "::warning%v::%s\n", pos, sprint(a))
}

// Warningf emits an warning message.
func (pos *ghaPos) Warningf(format string, a ...interface{}) {
	fmt.Fprintf(pos.w, "::warning%v::%s\n", pos, sprintf(format, a))
}

func (p *ghaPos) Format(f fmt.State, r rune) {
	if p.file == "" {
		return
	}
	fmt.Fprintf(f, " file=%s", p.file)
	if p.line == 0 {
		return
	}
	fmt.Fprintf(f, ",line=%d", p.line)
	if p.col == 0 {
		return
	}
	fmt.Fprintf(f, ",col=%d", p.col)
}

type ghaUnsafe string

func sprint(a []interface{}) ghaUnsafe {
	s := fmt.Sprintln(a...)
	return ghaUnsafe(s[:len(s)-1])
}

func sprintf(f string, a []interface{}) ghaUnsafe {
	return ghaUnsafe(fmt.Sprintf(f, a...))
}

func (s ghaUnsafe) Format(f fmt.State, r rune) {
	switch r {
	case 'q':
		f.Write([]byte{'"'})
		_, _ = ghaEscaper.WriteString(f, string(s))
		f.Write([]byte{'"'})

	case 'v', 's':
		_, _ = ghaEscaper.WriteString(f, string(s))
	}
}

type GitHubEvent map[string]interface{}

func (ghe GitHubEvent) String(log diag.Interface, path string) string {
	v := ghe.lookup(log, path)
	if s, ok := v.(string); ok {
		return s
	}
	diag.Errorf(log, "path %q=%v %[2]T not a string", path, v)
	return ""
}

func (ghe GitHubEvent) Int(log diag.Interface, path string) int {
	v := ghe.lookup(log, path)
	if v, ok := v.(float64); ok {
		return int(v)
	}
	diag.Errorf(log, "path %q=%v %[2]T not an int", path, v)
	return 0
}

func (ghe GitHubEvent) lookup(log diag.Interface, path string) interface{} {
	paths := strings.Split(path, ".")
	var src interface{} = (map[string]interface{})(ghe)
	for _, p := range paths {
		if v, ok := src.(map[string]interface{}); ok {
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			// diag.Debug(log, "lookup", p, "of", keys)
			if child, ok := v[p]; ok {
				src = child
				continue
			}
			diag.Warningf(log, "invalid event path %q (%s) in %T: %v", p, path, src, keys)
			return nil
		} else if v, ok := src.([]interface{}); ok {
			i, err := strconv.Atoi(p)
			if err != nil || i >= len(v) || i < 0 {
				diag.Warningf(log, "invalid event path %q (%s) in %T: 0..%d", p, path, src, len(v))
				return nil
			}
			src = v[i]
			continue
		}
	}
	return src
}

func appendFilef(path string, format string, a ...any) (int, error) {
	if path == "" {
		return 0, errEmptyPath
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o666)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return fmt.Fprintf(f, format, a...)
}
