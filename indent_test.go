package indent

import (
	"bytes"
	"io"
	"testing"
)

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

var prefix = "abcd"

func BenchmarkS2B(b *testing.B) {
	n := 0
	for i := 0; i < b.N; i++ {
		n += len(s2b(prefix))
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
