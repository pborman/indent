//   Copyright 2020 Paul Borman
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package indent

import (
	"bytes"
	"fmt"
	"io"
	"runtime"
	"runtime/debug"
	"testing"
)

func dup(s string) string {
	s1 := s + "x"
	return s1[:len(s)]
}

var heapMemory [100][]byte

func TestGC(t *testing.T) {
	input := []string{
		"abc",
		"def",
		"123456789",
	}
	var output [][]byte
	for _, s := range input {
		output = append(output, s2b(dup(s)))
	}
	defer func(p int) {
		debug.SetGCPercent(p)
	}(debug.SetGCPercent(1))
	for i := 0; i < 10000; i++ {
		for j, s := range input {
			if string(output[j]) != s {
				t.Fatalf("Round %d got %q want %q", i, output[j], s)
			}
		}
		heapMemory[i%100] = make([]byte, 4)
		runtime.GC()
	}
}

func TestIndent(t *testing.T) {
	for _, tt := range []struct {
		prefix string
		sol    bool
		in     string
		out    string
	}{
		{},
		{sol: true},
		{prefix: "--"},
		{prefix: "--", sol: true},
		{in: "ab", out: "ab"},
		{in: "ab\n", out: "ab\n"},
		{in: "ab", out: "ab", sol: true},
		{in: "ab\n", out: "ab\n", sol: true},

		{in: "ab", prefix: "--", out: "ab"},
		{in: "ab\n", prefix: "--", out: "ab\n"},
		{in: "ab\nc", prefix: "--", out: "ab\n--c"},
		{in: "ab\nc\n", prefix: "--", out: "ab\n--c\n"},

		{in: "ab", prefix: "--", sol: true, out: "--ab"},
		{in: "ab\n", prefix: "--", sol: true, out: "--ab\n"},
		{in: "ab\nc", prefix: "--", sol: true, out: "--ab\n--c"},
		{in: "ab\nc\n", prefix: "--", sol: true, out: "--ab\n--c\n"},
	} {
		// Try the base function first.
		out := string(indent(([]byte)(tt.in), ([]byte)(tt.prefix), tt.sol))
		if out != tt.out {
			t.Errorf("indentBytes(%q, %q, %v) got %q, want %q", tt.in, tt.prefix, tt.sol, out, tt.out)
		}

		// Only try the other functions when we are at the sol of a
		// line.
		if tt.sol {
			out = String(tt.prefix, tt.in)
			if out != tt.out {
				t.Errorf("String(%q, %q, %v) got %q, want %q", tt.in, tt.prefix, tt.sol, out, tt.out)
			}
			out := string(Bytes(([]byte)(tt.prefix), ([]byte)(tt.in)))
			if out != tt.out {
				t.Errorf("Bytes(%q, %q, %v) got %q, want %q", tt.in, tt.prefix, tt.sol, out, tt.out)
			}
		}
	}
}

func TestNewConsolidating(t *testing.T) {
	var w io.Writer = &bytes.Buffer{}

	// Any empty prefix should just return w.
	if New(w, "") != w {
		t.Error("New with no prefix returned a new writer")
	}
	if New(w, "a") == w {
		t.Error("New with prefix returned the old writer")
	}

	// A recursive New should combine the prefixes and keep the same w.
	w2 := New(New(w, "--"), "++").(*indenter)
	if string(w2.prefix) != "--++" {
		t.Errorf("Got prefix %q, want %q", w2.prefix, "--++")
	}
	if w2.w != w {
		t.Error("w2 did not inherit the io.Writer")
	}
}

func TestNew(t *testing.T) {

	for _, tt := range []struct {
		prefix string
		in     []string
		out    string
	}{
		{},
		{
			in:  []string{"ab"},
			out: "ab",
		}, {
			in:  []string{"ab\ncd\n"},
			out: "ab\ncd\n",
		}, {
			prefix: "--",
			in:     []string{"ab"},
			out:    "--ab",
		}, {
			prefix: "--",
			in:     []string{"ab\n"},
			out:    "--ab\n",
		}, {
			prefix: "--",
			in:     []string{"ab", "\n"},
			out:    "--ab\n",
		}, {
			prefix: "--",
			in:     []string{"ab", "", "\n"},
			out:    "--ab\n",
		}, {
			prefix: "--",
			in:     []string{"ab", "\ncd"},
			out:    "--ab\n--cd",
		}, {
			prefix: "--",
			in:     []string{"ab", "\ncd", ""},
			out:    "--ab\n--cd",
		},
	} {
		var buf bytes.Buffer
		w := New(&buf, tt.prefix)
		for _, s := range tt.in {
			if _, err := w.Write([]byte(s)); err != nil {
				t.Fatalf("write to bytes.buffer returned %v", err)
			}
		}
		out := buf.String()
		if out != tt.out {
			t.Errorf("New(%q).Write(%q) got %q, want %q", tt.prefix, tt.in, out, tt.out)
		}
	}
}

func TestNested(t *testing.T) {
	const (
		inner = "--"
		outer = "++"
	)
	for _, tt := range []struct {
		in1 string
		in2 string
		in3 string
		out string
	}{
		{
			in1: "ab\n",
			in2: "cd\n",
			in3: "ef\n",
			out: "--ab\n--++cd\n--ef\n",
		},
		{
			in1: "ab\n",
			in2: "cd",
			in3: "ef\n",
			out: "--ab\n--++cdef\n",
		},
		{
			in1: "ab",
			in2: "cd\n",
			in3: "ef\n",
			out: "--abcd\n--ef\n",
		},
		{
			in1: "ab",
			in2: "cd",
			in3: "ef\n",
			out: "--abcdef\n",
		},
	} {
		var buf bytes.Buffer
		w1 := New(&buf, inner)
		w1.Write([]byte(tt.in1))
		w2 := New(w1, outer)
		w2.Write([]byte(tt.in2))
		w1.Write([]byte(tt.in3))
		out := buf.String()
		if out != tt.out {
			t.Errorf("{%q,%q,%q} got %q, want %q", tt.in1, tt.in2, tt.in3, out, tt.out)
		}
	}

	// Do a couple double nesting
	for _, tt := range []struct {
		in1a, in1b string
		in2a, in2b string
		in3        string
		out        string
	}{
		{
			in1a: "1a\n",
			in2a: "2a\n",
			in3:  "3\n",
			in2b: "2b\n",
			in1b: "1b\n",
			out:  "++1a\n++--2a\n++--..3\n++--2b\n++1b\n",
		},
		{
			in1a: "1a\n",
			in2a: "2a\n",
			in3:  "3\n",
			in1b: "1b\n",
			out:  "++1a\n++--2a\n++--..3\n++1b\n",
		},
		{
			in1a: "1a",
			in2a: "2a",
			in3:  "3",
			in2b: "2b",
			in1b: "1b",
			out:  "++1a2a32b1b",
		},
	} {
		var buf bytes.Buffer
		w1 := New(&buf, "++")
		if tt.in1a != "" {
			w1.Write([]byte(tt.in1a))
		}
		w2 := New(w1, "--")
		if tt.in2a != "" {
			w2.Write([]byte(tt.in2a))
		}
		w3 := New(w2, "..")
		if tt.in3 != "" {
			w3.Write([]byte(tt.in3))
		}
		if tt.in2b != "" {
			w2.Write([]byte(tt.in2b))
		}
		if tt.in1b != "" {
			w1.Write([]byte(tt.in1b))
		}
		out := buf.String()
		if out != tt.out {
			t.Errorf("Nest %q got %q, want %q", []string{tt.in1a, tt.in2a, tt.in3, tt.in2b, tt.in1b}, out, tt.out)
		}
	}
}

func TestUnwrap(t *testing.T) {
	buf := &bytes.Buffer{}
	var w io.Writer = buf

	if uw := Unwrap(w, -1); uw != w {
		t.Error("Unwrapping unwrapped writer did not return original writer.")
	}
	fmt.Fprintln(w, "line 1")

	w1 := New(w, "1>");
	fmt.Fprintln(w1, "line 2")

	w2 := New(w1, "2>");
	fmt.Fprintln(w2, "line 3")

	if uw := Unwrap(w2, 0); uw != w2 {
		t.Error("Unwrap(w, 0) did not return w")
	}
	if uw := Unwrap(w2, 1); uw != w1 {
		t.Error("Unwrap(w2, 1) did not return w1")
	}
	if uw := Unwrap(w2, 2); uw != w {
		t.Error("Unwrap(w2, 2) did not return w")
	}

	fmt.Fprintln(w1, "line 4")
	fmt.Fprintln(w2, "line 5")
	want := `
line 1
1>line 2
1>2>line 3
1>line 4
1>2>line 5
`[1:]
	if got := buf.String(); got != want {
		t.Errorf("Mixing wrappers on newlines got:\n%s\nwant:\n%s",got, want)
	}
}

type fakeWriter struct {
	left int
	buf  bytes.Buffer
}

func (f *fakeWriter) Write(buf []byte) (int, error) {
	if f.left < len(buf) {
		f.buf.Write(buf[:f.left])
		n := f.left
		f.left = 0
		return n, io.EOF
	}
	f.buf.Write(buf)
	f.left -= len(buf)
	return len(buf), nil
}

// TestReturn makes sure we return the correct value according to the io.Writer
// contract.  We need to test writes both at the start of a line as well as
// writes starting at the middle of a line.
func TestReturn(t *testing.T) {
	input := []byte("abc\ndef\ngh")
	prefix := "--"

	for _, tt := range []struct {
		max int
		w0  int
		out int
	}{
		{max: 1, out: 0},
		{max: 2, out: 0},
		{max: 3, out: 1},
		{max: 4, out: 2},
		{max: 5, out: 3},
		{max: 6, out: 4},
		{max: 7, out: 4},
		{max: 8, out: 4},
		{max: 9, out: 5},
		{max: 10, out: 6},
		{max: 11, out: 7},
		{max: 12, out: 8},
		{max: 13, out: 8},

		{max: 3, w0: 1, out: 0},

		{max: 4, w0: 1, out: 1},
		{max: 4, w0: 2, out: 0},

		{max: 5, w0: 1, out: 2},
		{max: 5, w0: 2, out: 1},
		{max: 5, w0: 3, out: 0},

		{max: 6, w0: 1, out: 3},
		{max: 6, w0: 2, out: 2},
		{max: 6, w0: 3, out: 1},
		{max: 6, w0: 4, out: 0},

		{max: 7, w0: 1, out: 3},
		{max: 7, w0: 2, out: 2},
		{max: 7, w0: 3, out: 1},
		{max: 7, w0: 4, out: 0},

		{max: 8, w0: 1, out: 3},
		{max: 8, w0: 2, out: 2},
		{max: 8, w0: 3, out: 1},
		{max: 8, w0: 4, out: 0},

		{max: 9, w0: 1, out: 4},
		{max: 9, w0: 2, out: 3},
		{max: 9, w0: 3, out: 2},
		{max: 9, w0: 4, out: 1},
		{max: 9, w0: 5, out: 0},

		{max: 10, w0: 1, out: 5},
		{max: 10, w0: 2, out: 4},
		{max: 10, w0: 3, out: 3},
		{max: 10, w0: 4, out: 2},
		{max: 10, w0: 5, out: 1},
		{max: 10, w0: 6, out: 0},

		{max: 11, w0: 1, out: 6},
		{max: 11, w0: 2, out: 5},
		{max: 11, w0: 3, out: 4},
		{max: 11, w0: 4, out: 3},
		{max: 11, w0: 5, out: 2},
		{max: 11, w0: 6, out: 1},
		{max: 11, w0: 7, out: 0},

		{max: 12, w0: 1, out: 7},
		{max: 12, w0: 2, out: 6},
		{max: 12, w0: 3, out: 5},
		{max: 12, w0: 4, out: 4},
		{max: 12, w0: 5, out: 3},
		{max: 12, w0: 6, out: 2},
		{max: 12, w0: 7, out: 1},
		{max: 12, w0: 8, out: 0},

		{max: 13, w0: 1, out: 7},
		{max: 13, w0: 2, out: 6},
		{max: 13, w0: 3, out: 5},
		{max: 13, w0: 4, out: 4},
		{max: 13, w0: 5, out: 3},
		{max: 13, w0: 6, out: 2},
		{max: 13, w0: 7, out: 1},
		{max: 13, w0: 8, out: 0},

		{max: 14, w0: 1, out: 7},
		{max: 14, w0: 2, out: 6},
		{max: 14, w0: 3, out: 5},
		{max: 14, w0: 4, out: 4},
		{max: 14, w0: 5, out: 3},
		{max: 14, w0: 6, out: 2},
		{max: 14, w0: 7, out: 1},
		{max: 14, w0: 8, out: 0},

		{max: 15, w0: 1, out: 8},
		{max: 15, w0: 2, out: 7},
		{max: 15, w0: 3, out: 6},
		{max: 15, w0: 4, out: 5},
		{max: 15, w0: 5, out: 4},
		{max: 15, w0: 6, out: 3},
		{max: 15, w0: 7, out: 2},
		{max: 15, w0: 8, out: 1},
		{max: 15, w0: 9, out: 0},
	} {
		t.Run(fmt.Sprintf("Test %d:%d", tt.max, tt.w0), func(t *testing.T) {
			fw := &fakeWriter{left: tt.max}
			w := New(fw, prefix)
			n, _ := w.Write(input[:tt.w0])
			if n != tt.w0 {
				t.Errorf("Test %d:%d - w0 got %d, want %d <---------------", tt.max, tt.w0, n, tt.w0)
				return
			}
			out, _ := w.Write(input[tt.w0:])
			if out != tt.out {
				t.Errorf("Test %d:%d - got %d, want %d <---------------", tt.max, tt.w0, out, tt.out)
			}
		})
	}
}

var tprefix = "abcd"

func BenchmarkS2B(b *testing.B) {
	n := 0
	for i := 0; i < b.N; i++ {
		n += len(s2b(tprefix))
	}
}

var (
	prefix1      = string(make([]byte, 1))
	prefix10     = string(make([]byte, 10))
	prefix1000   = string(make([]byte, 1000))
	prefix10000  = string(make([]byte, 10000))
	prefix100000 = string(make([]byte, 100000))
)

func BenchmarkCopy1(b *testing.B) {
	n := 0
	for i := 0; i < b.N; i++ {
		n += len([]byte(prefix1))
	}
}

func BenchmarkCopy10(b *testing.B) {
	n := 0
	for i := 0; i < b.N; i++ {
		n += len([]byte(prefix10))
	}
}

func BenchmarkCopy1000(b *testing.B) {
	n := 0
	for i := 0; i < b.N; i++ {
		n += len([]byte(prefix1000))
	}
}

func BenchmarkCopy10000(b *testing.B) {
	n := 0
	for i := 0; i < b.N; i++ {
		n += len([]byte(prefix10000))
	}
}

func BenchmarkCopy100000(b *testing.B) {
	n := 0
	for i := 0; i < b.N; i++ {
		n += len([]byte(prefix100000))
	}
}
