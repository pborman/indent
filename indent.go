// Package indent indents lines of text with a prefix.  The New function is used
// to return a writer that indents the lines written to it. For example:
//
//		var buf bytes.Buffer
//		w := indent.New(&buf, "> ")
//	 	w.Write([]byte(`line 1
//	line 2
//	line 3
//	`)
//
// will result in
//
//	> line 1
//	> line 2
//	> line 3
//
// Indenters may be nested:
//
//		var buf bytes.Buffer
//		w := indent.New(&buf, "> ")
//	 	w.Write([]byte("line 1\n"))
//		nw := indent.New(w, "..")
//	 	nw.Write([]byte("line 2\n"))
//	 	w.Write([]byte("line 3\n"))
//
// will result in
//
//	> line 1
//	> ..line 2
//	> line 3
//
// The String and Bytes functions are optimized equivalents of
//
//	var buf bytes.Buffer()
//	indent.New(&buf, prefix).Write(input)
//	return buf.String() // or buf.Bytes()
package indent

import (
	"bytes"
	"io"
	"reflect"
	"unsafe"
)

// Using unsafe here is both safe and significantly faster.  On a MacBook Pro
// with 2.9GHz Intel Core i9 processor the routines take just under 0.5ns
// regardless of the length..  Converting strings of length 1, 10, 1000, 10,000,
// and 100,000 took around 5, 5, 125, 700, and 6,400ns respectively.
//
// These are safe in this package as we assure that a byte slice made from the
// string is never modified and after we make a string from a byte slice the
// original byte slice is never modified.  These functions are not safe for
// general use.

// s2b returns a []byte, that points to s.  The contents of the returned
// slice must not be modified.
func s2b(s string) []byte {
	// A string has a 2 word header and a byte slice has a 3 word header.
	var b []byte
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	bh.Data = sh.Data
	bh.Len = sh.Len
	bh.Cap = sh.Len
	return b
}

// b2s turns b into a string without copying.  The contents of b must not be
// modified after this.
func b2s(b []byte) string { return *(*string)(unsafe.Pointer(&b)) }

// String returns input with each line in input prefixed by prefix.
func String(prefix, input string) string {
	if len(input) == 0 || len(prefix) == 0 {
		return input
	}
	return b2s(indent(s2b(input), s2b(prefix), true))
}

// Bytes returns input with each line in input prefixed by prefix.
func Bytes(prefix, input []byte) []byte {
	if len(input) == 0 || len(prefix) == 0 {
		return input
	}
	return indent(input, prefix, true)
}

type indenter struct {
	w      io.Writer
	prefix []byte
	sol    bool      // true if we are at the start of a line
	p      *indenter // the indenter that wrapped us
}

// NewWriter is the name used in github.com/openconfig/goyang/pkg/indent.
var NewWriter = New

// New returns a writer that will prefix all lines written to it with prefix and
// then writes the results to w.  New is intelligent about recursive calls to
// New.  New return w if prefix is the empty string.  When nesting, New does not
// assume it is at the start of a line, it maintains this information as you
// nest and unwind indenters.  It normally is best to only transition between
// nested writers after a newline has been written.
func New(w io.Writer, prefix string) io.Writer {
	if len(prefix) == 0 {
		return w
	}
	// If we are indenting an indenter then we can just combine the
	// indents.
	if in, ok := w.(*indenter); ok {
		in.p = &indenter{
			w:      in.w,
			prefix: append(in.prefix, prefix...),
			sol:    in.sol,
		}
		return in.p
	}
	return &indenter{
		w:      w,
		prefix: []byte(prefix),
		sol:    true,
	}
}

func (in *indenter) Write(buf []byte) (int, error) {
	// If we were wrapped then try to preserve the sol bit.
	// This assume proper nesting.
	if p := in.p; p != nil {
		for p.p != nil {
			p = p.p
		}
		in.sol = p.sol
		in.p = nil
	}

	if len(buf) == 0 {
		return 0, nil
	}
	nbuf := indent(buf, in.prefix, in.sol)
	r, err := in.w.Write(nbuf)
	if r == len(nbuf) {
		in.sol = nbuf[r-1] == '\n'
		return len(buf), err
	}

	// The write failed someplace.  Figure out
	// how much of what we wrote came from buf
	// and return that amount.

	nbuf = nbuf[:r]

	if r == 0 {
		return 0, err
	}

	// If sol was true then we started with a
	// prefix, if not, we did not.  So strip
	// the initial prefix if we wrote one.
	if in.sol {
		r -= len(in.prefix)
		if r <= 0 {
			in.sol = true
			return 0, err
		}
		nbuf = nbuf[len(in.prefix):]
	}

	nl := bytes.Count(nbuf, []byte{'\n'})
	if nl == 0 {
		// There are no newlines so there are no
		// prefixes left to accoubt for.
		in.sol = buf[r-1] == '\n'
		return r, err
	}

	// Find how much we wrote up to and including the last newline
	ln := bytes.LastIndex(nbuf, []byte{'\n'})
	r = ln - (nl-1)*len(in.prefix) + 1

	// Now figure out how many bytes were after the last newline.
	// If more than our prefix then add those back into the total
	// number of bytes read from buf.
	x := len(nbuf) - ln - 1
	if x > len(in.prefix) {
		r += x - len(in.prefix)
	}
	in.sol = buf[r-1] == '\n'
	return r, err
}

// indent returns buf with each line prefixed by prefix.  The sol flag indicates
// if we are at the start of a line.
func indent(buf, prefix []byte, sol bool) []byte {
	if len(buf) == 0 || len(prefix) == 0 {
		return buf
	}
	lines := bytes.SplitAfter(buf, []byte{'\n'})
	n := len(lines) - 1
	// If buf ends in a newline there will be a zero slice at the
	// of the the lines.  Needs to be removed so we don't append
	// and extra prefix.
	if len(lines[n]) == 0 {
		lines = lines[:n]
		n--
	}
	n = len(buf) + n*len(prefix)
	if sol {
		n += len(prefix)
	}
	buf = make([]byte, n)

	n = 0
	if !sol {
		n = copy(buf, lines[0])
		lines = lines[1:]
	}
	for _, line := range lines {
		n += copy(buf[n:], prefix)
		n += copy(buf[n:], line)
	}
	return buf
}
