package codegen

import (
	"crypto/sha256"
	"encoding/base32"
	"go/format"
	"strconv"
	"strings"
	"unsafe"
)

type stringerSet []*stringer

type stringer struct {
	unknownValue bool     // whether to stringify unknown values
	unknownLabel bool     // whether add the type name to unknown values
	method       bool     // whether to generate a method instead of a global func
	unsigned     bool     // whether typ is unsigned
	typ          string   // type name (default: int)
	tvar         string   // variable name to use for type (default: first letter of typ)
	fn           string   // func name (default: String)
	doc          []string // doc comment lines
	offset       int      // offset of first value
	values       []string // values
}

// Add adds a new string function.named fn on typ, using tvar as the argument
// name.
func (ss *stringerSet) Add(fn, typ, tvar string, signed bool) *stringer {
	s := new(stringer)
	s.method = true
	s.fn = fn
	s.typ = typ
	s.tvar = tvar
	s.unsigned = !signed
	*ss = append(*ss, s)
	return s
}

// Global makes the function into a global function rather than a method.
func (s *stringer) Global() *stringer {
	s.method = false
	return s
}

// Default chooses whether to include the type name and stringified value in the
// returned string if the value is out of range.
func (s *stringer) Default(label, value bool) *stringer {
	s.unknownLabel = label
	s.unknownValue = value
	return s
}

// Doc adds a line to the function's godoc.
func (s *stringer) Doc(lines ...string) *stringer {
	s.doc = append(s.doc, lines...)
	return s
}

// Set sets a value, updating the range of the stringer as necessary.
func (s *stringer) Set(i int, value string) {
	if s.unsigned && i < 0 {
		panic("cannot add negative value to unsigned stringer")
	}
	if d := s.offset - i; d > 0 {
		s.values = append(make([]string, d), s.values...)
	}
	if d := 1 + i - s.offset - len(s.values); d > 0 {
		s.values = append(s.values, make([]string, d)...)
	}
	s.values[i-s.offset] = value
}

// Bytes returns the source code for the stringer.
func (ss stringerSet) Bytes() (b []byte) {
	const iprefix = "_stringer_" // internal name prefix

	s := make([]*stringer, len(ss))
	for i, st := range ss {
		// shallow copy
		st := *st

		// optimize out empty values at the beginning
		for len(st.values) != 0 && st.values[0] == "" {
			st.values = st.values[1:]
			st.offset++
		}

		// optimize out empty values at the end
		for len(st.values) != 0 && st.values[len(st.values)-1] == "" {
			st.values = st.values[:len(st.values)-1]
		}

		// append the copy
		s[i] = &st
	}

	var maxlen, totlen int
	for _, st := range s {
		for _, v := range st.values {
			n := len(v)
			if n > maxlen {
				maxlen = n
			}
			totlen += n
		}
	}

	var (
		npfx int                // number of prefixes
		pfxd []byte             // prefix data
		pfxn []int              // used prefix bytes
		pfxi = map[string]int{} // prefix index
		pfxo = map[string]int{} // value data offset
	)
	for _, st := range s {
		for _, v := range st.values {
			if _, ok := pfxo[v]; ok {
				continue
			}

			if v == "" {
				pfxi[v] = -1
				pfxo[v] = -1
				continue
			}

			// find the longest prefix
			pi, pn := -1, 0
			for n := len(v); n > 0; n-- {
				if i, ok := pfxi[v[:n]]; ok {
					pi, pn = i, n
					break
				}
			}

			// add a new prefix if none was found
			if pi == -1 {
				pi = npfx
				pfxd = append(pfxd, make([]byte, maxlen)...)
				pfxn = append(pfxn, 0)
				npfx++
			}

			// update the prefix cache
			for i := 1; i < len(v); i++ {
				delete(pfxi, v[:i])
			}
			pfxi[v] = pi

			// add additional prefix bytes
			for i := pn; i < len(v); i++ {
				pfxd[maxlen*pi+i] = v[i]
				pfxo[v] = maxlen * pi
			}
			pfxn[pi] = len(v)
		}
	}

	var pfxdn int // used prefix data
	for _, pn := range pfxn {
		// shift unused bytes out of the prefix data
		copy(pfxd[pfxdn+pn:], pfxd[pfxdn+maxlen:])
		pfxd = pfxd[:len(pfxd)-maxlen+pn]

		// update offsets
		for v, o := range pfxo {
			if o >= pfxdn+maxlen {
				pfxo[v] -= maxlen - pn
			}
		}
		pfxdn += pn
	}

	pfxds := *(*string)(unsafe.Pointer(&pfxd)) // use the trick from strings.Builder so we don't need copy the entire data slice
	for _, st := range s {
		for _, v := range st.values {
			if po, ok := pfxo[v]; !ok {
				panic("sanity check failed: value not in prefix data index: " + strconv.Quote(v) + " not found")
			} else if po == -1 {
				if v != "" {
					panic("sanity check failed: value data incorrect: got empty string, expected " + strconv.Quote(v))
				}
			} else if pd := pfxds[po:]; len(pd) < len(v) {
				panic("sanity check failed: value data incorrect: data too short")
			} else if pd = pd[:len(v)]; pd != v {
				panic("sanity check failed: value data incorrect: got " + strconv.Quote(pd) + ", expected " + strconv.Quote(v))
			}
		}
	}

	sh := sha256.Sum256(pfxd)
	id := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sh[:len(sh)/6]))

	sn := make([]string, len(s)) // stringer data names
	for i, st := range s {
		if st.fn == "" {
			st.fn = "String"
		}
		if st.typ == "" {
			if st.unsigned {
				st.typ = "uint"
			} else {
				st.typ = "int"
			}
		}
		if st.tvar == "" {
			st.tvar = strings.ToLower(st.typ[:1])
		}
		if st.method {
			sn[i] = st.typ + "_" + st.fn
		} else {
			sn[i] = "global_" + st.typ + "_" + st.fn
		}
		if st.unsigned && st.offset < 0 {
			panic("stringer for unsigned type must not have negative offset")
		}
	}

	for i, st := range s {
		for _, line := range st.doc {
			b = append(b, "\n// "...)
			b = append(b, line...)
		}
		if st.method {
			b = append(b, "\nfunc ("...)
			b = append(b, st.tvar...)
			b = append(b, " "...)
			b = append(b, st.typ...)
			b = append(b, ") "...)
			b = append(b, st.fn...)
			b = append(b, "() string {\n"...)
		} else {
			b = append(b, "\nfunc "...)
			b = append(b, st.fn...)
			b = append(b, "("...)
			b = append(b, st.tvar...)
			b = append(b, " "...)
			b = append(b, st.typ...)
			b = append(b, ") string {\n"...)
		}
		if n := len(st.values); n != 0 {
			b = append(b, "\tif o := int64("...)
			b = append(b, st.tvar...)
			b = append(b, ")*2"...)
			if st.offset != 0 {
				b = append(b, "-"...)
				b = strconv.AppendInt(b, int64(st.offset)*2, 10)
			}
			b = append(b, "; "...)
			b = append(b, "o >= 0 && "...)
			b = append(b, "o < "...)
			b = strconv.AppendInt(b, int64(len(st.values))*2-1, 10)
			b = append(b, " {\n\t\treturn "...)
			b = append(b, iprefix...)
			b = append(b, id...)
			b = append(b, "["...)
			b = append(b, iprefix...)
			b = append(b, sn[i]...)
			b = append(b, "[o]:"...)
			b = append(b, iprefix...)
			b = append(b, sn[i]...)
			b = append(b, "[o+1]]\n\t}\n"...)

			b = append(b, "\treturn \""...)
			if st.unknownLabel {
				b = append(b, st.typ...)
			}
			if st.unknownValue {
				b = append(b, "(\" + strconv.Format"...)
				if st.unsigned {
					b = append(b, "Uint(uint64("...)
				} else {
					b = append(b, "Int(int64("...)
				}
				b = append(b, st.tvar...)
				b = append(b, "), 10) + \")"...)
			}
			b = append(b, "\"\n"...)
		} else {
			b = append(b, "\treturn \"\"\n"...)
		}
		b = append(b, "}\n"...)
	}

	b = append(b, "\nconst "...)
	b = append(b, iprefix...)
	b = append(b, id...)
	b = append(b, " = "...)
	b = strconv.AppendQuote(b, pfxds)
	b = append(b, " // ratio "...)
	b = strconv.AppendInt(b, int64(len(pfxds)), 10)
	b = append(b, " / "...)
	b = strconv.AppendInt(b, int64(totlen), 10)
	b = append(b, " = "...)
	b = strconv.AppendFloat(b, float64(10*len(pfxds)/totlen)/10, 'f', 1, 64)
	b = append(b, '\n')

	for i, st := range s {
		if len(st.values) != 0 {
			b = append(b, "var "...)
			b = append(b, iprefix...)
			b = append(b, sn[i]...)
			b = append(b, " = [...]int{"...)
			for i, v := range st.values {
				if i != 0 {
					b = append(b, ", "...)
				}
				if o := pfxo[v]; o != -1 {
					b = strconv.AppendInt(b, int64(o), 10)
					b = append(b, ", "...)
					b = strconv.AppendInt(b, int64(o+len(v)), 10)
				} else {
					b = append(b, "0, 0"...)
				}
			}
			b = append(b, "}\n"...)
		}
	}

	if f, err := format.Source(b); err != nil {
		panic(err)
	} else {
		b = f
	}
	return
}
