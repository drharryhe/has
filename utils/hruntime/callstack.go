// 代码主要来自 github.com/pkg/errors 中callstack的实现，并根据需要进行了修改

package hruntime

import (
	"fmt"
	"io"
	"path"
	"runtime"
	"strconv"
	"strings"
)

const (
	maxStackDepth = 32
)

func SprintCallStack(depth, skip int) string {
	st := callers(depth, skip)
	return fmt.Sprintf("%+v", st)
}

func SprintfCallStack(f string, depth, skip int) string {
	st := callers(depth, skip)
	return fmt.Sprintf(f, st)
}

func PrintCallStack(depth, skip int) {
	st := callers(depth, skip)
	fmt.Printf("%+v", st)
}

func PrintfCallStack(f string, depth, skip int) {
	st := callers(depth, skip)
	fmt.Printf(f, st)
}

func SprintCallers(depth, skip int) []string {
	st := callers(depth, skip)
	sts := st.stackTrace()
	var ss []string
	for _, s := range sts {
		ss = append(ss, fmt.Sprintf("%+v", s))
	}

	return ss
}

func SprintfCallers(f string, depth, skip int) []string {
	st := callers(depth, skip)
	sts := st.stackTrace()
	var ss []string
	for _, s := range sts {
		ss = append(ss, fmt.Sprintf(f, s))
	}

	return ss
}

// frame represents a program counter inside a stack frame.
// For historical reasons if frame is interpreted as a uintptr
// its value represents the program counter + 1.
type frame uintptr

// pc returns the program counter for this frame;
// multiple frames may have the same PC value.
func (f frame) pc() uintptr { return uintptr(f) - 1 }

// file returns the full path to the file that contains the
// function for this frame's pc.
func (f frame) file() string {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return "unknown"
	}
	file, _ := fn.FileLine(f.pc())
	return file
}

// line returns the line number of source code of the
// function for this frame's pc.
func (f frame) line() int {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return 0
	}
	_, line := fn.FileLine(f.pc())
	return line
}

// name returns the name of this function, if known.
func (f frame) name() string {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return "unknown"
	}
	return fn.Name()
}

// Format formats the frame according to the fmt.Formatter interface.
//
//    %s    source file
//    %d    source line
//    %n    function name
//    %v    equivalent to %s:%d
//
// Format accepts flags that alter the printing of some verbs, as follows:
//
//    %+s   function name and path of source file relative to the compile time
//          GOPATH separated by \n\t (<funcName>\n\t<path>)
//    %+v   equivalent to %+s:%d
func (f frame) Format(s fmt.State, verb rune) {
	switch verb {
	case 's':
		switch {
		case s.Flag('+'):
			io.WriteString(s, f.name())
			io.WriteString(s, "\t\t --> \t\t")
			io.WriteString(s, f.file())
		default:
			io.WriteString(s, path.Base(f.file()))
		}
	case 'd':
		io.WriteString(s, strconv.Itoa(f.line()))
	case 'n':
		io.WriteString(s, funcName(f.name()))
	case 'v':
		f.Format(s, 's')
		io.WriteString(s, ":")
		f.Format(s, 'd')
	}
}

// MarshalText formats a stackTrace frame as a htext string. The output is the
// same as that of fmt.Sprintf("%+v", f), but without newlines or tabs.
func (f frame) MarshalText() ([]byte, error) {
	name := f.name()
	if name == "unknown" {
		return []byte(name), nil
	}
	return []byte(fmt.Sprintf("%s %s:%d", name, f.file(), f.line())), nil
}

// stackTrace is stack of frames from innermost (newest) to outermost (oldest).
type stackTrace []frame

// stack represents a stack of program counters.
type stack []uintptr

func (s *stack) Format(st fmt.State, verb rune) {
	switch verb {
	case 'v':
		switch {
		case st.Flag('+'):
			for _, pc := range *s {
				f := frame(pc)
				fmt.Fprintf(st, "\n%+v", f)
			}
		}
	}
}

func (s *stack) stackTrace() stackTrace {
	f := make([]frame, len(*s))
	for i := 0; i < len(f); i++ {
		f[i] = frame((*s)[i])
	}
	return f
}

func callers(dep, skip int) *stack {
	var pcs [maxStackDepth]uintptr
	n := runtime.Callers(skip, pcs[:dep])
	var st stack = pcs[0:n]
	st = st[0 : len(st)-2]
	return &st
}

// funcName removes the path prefix component of a function's name reported by func.Name().
func funcName(name string) string {
	i := strings.LastIndex(name, "/")
	name = name[i+1:]
	i = strings.Index(name, ".")
	return name[i+1:]
}
