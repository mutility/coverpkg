// Package diag contains interfaces and utility functions to ease production
// of diagnostics. Implementations are suggested to provide at least the
// methods of diag.Interface, as it will be common for entry points to receive
// a diag.Interface. Additional methods will be leveraged where available.
//
// Typical use in a function that wants to provide diagnostics looks like this:
//
//     func Foo(log diag.Interface) {
// 	       diag.Debugf(log, "Hello %s!", "World")
//     }
//
// It's also okay to accept a diag.Debugger, diag.Errorer, or diag.Warninger
// when Foo and what it calls will use only use a subset of the capabilities.
//
// New() enables a trivial implementation around existing io.Writers, such as
// os.Stdout, os.Stderr, etc. This is useful for main or testing packages.
package diag

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type (
	Debugger  interface{ Debug(...interface{}) }
	Debugfer  interface{ Debugf(string, ...interface{}) }
	Errorer   interface{ Error(...interface{}) }
	Errorfer  interface{ Errorf(string, ...interface{}) }
	ErrorAter interface {
		ErrorAt(string, int, int, ...interface{})
	}
	ErrorAtfer interface {
		ErrorAtf(string, int, int, string, ...interface{})
	}
	Warninger   interface{ Warning(...interface{}) }
	Warningfer  interface{ Warningf(string, ...interface{}) }
	WarningAter interface {
		WarningAt(string, int, int, ...interface{})
	}
	WarningAtfer interface {
		WarningAtf(string, int, int, string, ...interface{})
	}
)

// Interface includes the three core diagnostic methods. All functions in diag
// can function on top of these.
type Interface interface {
	Debugger
	Errorer
	Warninger
}

// Context merges Interface and context.Context, enabling a single parameter to
// serve both purposes.
type Context interface {
	Interface
	context.Context
}

// FullInterface includes all diagnostic methods. This interface may be
// extended at any point, so using it confers no compatibility guarantees.
// It should be used primarily for testing if your interface is complete.
//
//     func TestFullDiagInterface(t *testing.T) {
//         var impl interface{} = (*myimpl)(nil)
//         if _, ok := impl.(diag.FullInterface); !ok {
//	           t.Error("myimpl doesn't implement diag.FullInterface")
//         }
//     }
//
// Using it any other way risks breaking your code rather than just your tests.
// For instance, the following will yield a build error for any missing methods.
//
//     var _ diag.FullInterface = (*myimpl)(nil)
//
type FullInterface interface {
	Interface
	Debugfer
	Errorfer
	ErrorAter
	ErrorAtfer
	Warningfer
	WarningAter
	WarningAtfer
}

// New creates an Interface wrapper for an io.Writer. Set optional DebugPrefix,
// WarningPrefix and ErrorPrefix strings on the returned value. Set WDebug,
// WWarning, or WError io.Writers on the returned value.
func New(w io.Writer) *wrap {
	if w == nil {
		w = os.Stderr
	}
	return &wrap{io.Discard, w, w, "[D]", "[W]", "[E]"}
}

// New creates an Interface wrapper for an io.Writer. Set optional DebugPrefix,
// WarningPrefix and ErrorPrefix strings on the returned value. Set WDebug,
// WWarning, or WError io.Writers on the returned value.
func NewDebug(w io.Writer) *wrap {
	if w == nil {
		w = os.Stderr
	}
	return &wrap{w, w, w, "[D]", "[W]", "[E]"}
}

type wrap struct {
	WDebug        io.Writer
	WWarning      io.Writer
	WError        io.Writer
	DebugPrefix   string
	WarningPrefix string
	ErrorPrefix   string
}

func (w *wrap) Debug(a ...interface{}) {
	fmt.Fprintln(w.WDebug, append([]interface{}{w.DebugPrefix}, a...)...)
}

func (w *wrap) Warning(a ...interface{}) {
	fmt.Fprintln(w.WWarning, append([]interface{}{w.WarningPrefix}, a...)...)
}

func (w *wrap) Error(a ...interface{}) {
	fmt.Fprintln(w.WWarning, append([]interface{}{w.ErrorPrefix}, a...)...)
}

// WithContext creates an Interface wrapper with a Context.
func WithContext(ctx context.Context, i Interface) Context {
	return &wrapContext{ctx, i}
}

type wrapContext struct {
	context.Context
	Interface
}

// Debug outputs a debug message, unless d is nil.
func Debug(d Debugger, a ...interface{}) {
	if d != nil {
		d.Debug(a...)
	}
}

// Debugf outputs a formatted debug message, unless d is nil.
func Debugf(d Debugger, format string, a ...interface{}) {
	if df, ok := d.(Debugfer); ok {
		df.Debugf(format, a...)
	} else if d != nil {
		d.Debug(fmt.Sprintf(format, a...))
	}
}

// Error outputs an error message, unless e is nil.
func Error(e Errorer, a ...interface{}) {
	if e != nil {
		e.Error(a...)
	}
}

// Errorf outputs a formatted error message, unless e is nil.
func Errorf(e Errorer, format string, a ...interface{}) {
	if ef, ok := e.(Errorfer); ok {
		ef.Errorf(format, a...)
	} else if e != nil {
		e.Error(fmt.Sprintf(format, a...))
	}
}

// ErrorAt outputs an error message with location, unless e is nil.
func ErrorAt(e Errorer, file string, line, col int, a ...interface{}) {
	if ea, ok := e.(ErrorAter); ok {
		ea.ErrorAt(file, line, col, a...)
	} else if e != nil {
		e.Error(at(file, line, col, a)...)
	}
}

// ErrorAtf outputs a formatted error message with location, unless e is nil.
func ErrorAtf(e Errorer, file string, line, col int, format string, a ...interface{}) {
	if eaf, ok := e.(ErrorAtfer); ok {
		eaf.ErrorAtf(file, line, col, format, a...)
	} else if ea, ok := e.(ErrorAter); ok {
		ea.ErrorAt(file, line, col, fmt.Sprintf(format, a...))
	} else if ef, ok := e.(Errorfer); ok {
		ef.Errorf(atf(file, line, col, format), a...)
	} else if e != nil {
		e.Error(fmt.Sprintf(atf(file, line, col, format), a...))
	}
}

// Warning outputs an warning message, unless w is nil.
func Warning(w Warninger, a ...interface{}) {
	if w != nil {
		w.Warning(a...)
	}
}

// Warningf outputs a formatted warning message, unless w is nil.
func Warningf(w Warninger, format string, a ...interface{}) {
	if wf, ok := w.(Warningfer); ok {
		wf.Warningf(format, a...)
	} else if w != nil {
		w.Warning(fmt.Sprintf(format, a...))
	}
}

// WarningAt outputs an warning message with location, unless w is nil.
func WarningAt(w Warninger, file string, line, col int, a ...interface{}) {
	if wa, ok := w.(WarningAter); ok {
		wa.WarningAt(file, line, col, a...)
	} else if w != nil {
		w.Warning(at(file, line, col, a)...)
	}
}

// WarningAtf outputs a formatted warning message with location, unless w is nil.
func WarningAtf(w Warninger, file string, line, col int, format string, a ...interface{}) {
	if waf, ok := w.(WarningAtfer); ok {
		waf.WarningAtf(file, line, col, format, a...)
	} else if wa, ok := w.(WarningAter); ok {
		wa.WarningAt(file, line, col, fmt.Sprintf(format, a...))
	} else if wf, ok := w.(Warningfer); ok {
		wf.Warningf(atf(file, line, col, format), a...)
	} else if w != nil {
		w.Warning(fmt.Sprintf(atf(file, line, col, format), a...))
	}
}

func at(file string, line, col int, a []interface{}) []interface{} {
	loc := "[" + file
	if line != 0 {
		loc += ":" + strconv.Itoa(line)
		if col != 0 {
			loc += "." + strconv.Itoa(col)
		}
	}
	loc += "]"
	return append([]interface{}{loc}, a...)
}

func atf(file string, line, col int, format string) string {
	loc := "[" + strings.ReplaceAll(file, "%", "%%")
	if line != 0 {
		loc += ":" + strconv.Itoa(line)
		if col != 0 {
			loc += "." + strconv.Itoa(col)
		}
	}
	loc += "] " + format
	return loc
}
