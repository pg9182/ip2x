// Package ip2x reads IP2Location binary databases.
package ip2x

import (
	"bytes"
	"errors"
	"io"
	"net/netip"
	"strconv"
	"strings"
	"unsafe"
)

// DBProduct represents an IP2Location database product.
type DBProduct uint8

// String returns the name of the product.
func (p DBProduct) String() string {
	return p.product()
}

// FormatProduct returns the full product name of t.
func (p DBProduct) FormatProduct(t DBType) string {
	return p.product() + " " + p.FormatType(t)
}

// FormatType prepends the product prefix to t.
func (p DBProduct) FormatType(t DBType) string {
	return p.prefix() + t.String()
}

// SupportsType returns true if p supports variant t.
func (p DBProduct) SupportsType(t DBType) bool {
	return p != 0 && t != 0 && p < dbProductUpper && t < dbTypeUpper && dbexpcols[t][p] != 0
}

// DBType represents an IP2Location database variant. Each database type
// contains different sets of columns.
type DBType uint8

// String formats the type as a string.
func (t DBType) String() string {
	return strconv.Itoa(int(t))
}

// DBField represents a database column.
type DBField int

// String returns the name of the database column.
func (f DBField) String() string {
	return f.column()
}

// DB reads an IP2Location binary database.
type DB struct {
	r io.ReaderAt

	// header
	dbtype   DBType
	dbcolumn uint8
	dbyear   uint8
	dbmonth  uint8
	dbday    uint8
	ip4count uint32
	ip4base  uint32
	ip6count uint32
	ip6base  uint32
	ip4idx   uint32
	ip6idx   uint32
	prcode   DBProduct
	prtype   uint8
	filesize uint32
}

// New opens an IP2Location binary database reading from r.
func New(r io.ReaderAt) (*DB, error) {
	var db DB
	var row [64]byte // 64-byte header
	if _, err := r.ReadAt(row[:], 0); err == nil {
		db.r = r
		db.dbtype, db.dbcolumn = DBType(row[0]), row[1]
		db.dbyear, db.dbmonth, db.dbday = row[2], row[3], row[4]
		db.ip4count, db.ip4base = as_le_u32(row[5:]), as_le_u32(row[9:])
		db.ip6count, db.ip6base = as_le_u32(row[13:]), as_le_u32(row[17:])
		db.ip4idx, db.ip6idx = as_le_u32(row[21:]), as_le_u32(row[25:])
		db.prcode, db.prtype = DBProduct(row[29]), row[30]
		db.filesize = as_le_u32(row[31:])
	} else {
		return nil, err
	}
	if row[0] == 'P' && row[1] == 'K' {
		return nil, errors.New("database is zipped")
	}
	if db.dbmonth == 0 || db.dbmonth > 12 || db.dbday == 0 || db.dbday > 31 {
		return nil, errors.New("database is corrupt")
	}
	if db.dbyear < 21 {
		// only has prcode field in >= 2021
		return nil, errors.New("database is too old (date: " + db.Version() + ")")
	}
	if !db.prcode.SupportsType(db.dbtype) {
		return nil, errors.New("unsupported database " + db.prcode.FormatProduct(db.dbtype))
	}
	if ecol := dbexpcols[db.dbtype][db.prcode]; db.dbcolumn != ecol {
		return nil, errors.New("database is corrupt or library is buggy: db " + db.prcode.FormatProduct(db.dbtype) + ": expected " + strconv.Itoa(int(ecol)) + "  cols, got " + strconv.Itoa(int(db.dbcolumn)))
	}
	return &db, nil
}

// String returns a human-readable string describing the database.
func (db *DB) String() string {
	var s strings.Builder
	s.WriteString(db.prcode.FormatProduct(db.dbtype))
	s.WriteByte(' ')
	s.WriteString(db.Version())
	s.WriteByte(' ')
	s.WriteByte('[')
	for n, f := 0, DBField(1); f < dbFieldUpper; f++ {
		if db.Has(f) {
			if n != 0 {
				s.WriteByte(',')
			}
			s.WriteString(f.String())
			n++
		}
	}
	s.WriteByte(']')
	s.WriteByte(' ')
	s.WriteByte('(')
	if v4, v6 := db.HasIPv4(), db.HasIPv6(); v4 && !v6 {
		s.WriteString("IPv4")
	} else if !v4 && v6 {
		s.WriteString("IPv6")
	} else {
		s.WriteString("IPv4+IPv6")
	}
	s.WriteByte(')')
	return s.String()
}

// Info returns the database product and type.
func (db *DB) Info() (DBProduct, DBType) {
	return db.prcode, db.dbtype
}

// Version returns the database version.
func (db *DB) Version() string {
	b := []byte{
		'2', '0',
		'0' + db.dbyear/10%10,
		'0' + db.dbyear%10,
		'-',
		'0' + db.dbmonth/10%10,
		'0' + db.dbmonth%10,
		'-',
		'0' + db.dbday/10%10,
		'0' + db.dbday%10,
	}
	return as_strref_unsafe(b)
}

// Has returns true if the database contains f.
func (db *DB) Has(f DBField) bool {
	return dbfds[db.dbtype][db.prcode][f].IsValid()
}

// HasIPv4 returns true if the database contains IPv4 entries.
func (db *DB) HasIPv4() bool {
	return db.ip4count != 0
}

// HasIPv6 returns true if the database contains HasIPv6 entries.
func (db *DB) HasIPv6() bool {
	return db.ip6count != 0
}

// EachField calls fn for each column in the database until fn returns false.
func (db *DB) EachField(fn func(DBField) bool) {
	if fn != nil {
		for f := DBField(1); f < dbFieldUpper; f++ {
			if db.Has(f) {
				if !fn(f) {
					return
				}
			}
		}
	}
}

// LookupString parses and looks up a in db. If a parse error occurs, an empty
// record and nil error is returned. To catch parse errors, parse it separately
// using [net/netip.ParseAddr], and pass it to [DB.Lookup].
func (db *DB) LookupString(ip string) (r Record, err error) {
	a, _ := netip.ParseAddr(ip)
	return db.Lookup(a)
}

// Lookup looks up a in db. If a is not found, an empty record and nil error is
// returned. If an i/o error occurs, an empty record and non-nil error is
// returned.
func (db *DB) Lookup(a netip.Addr) (r Record, err error) {
	if !a.IsValid() {
		return
	}

	// unmap the ip address into a native v4/v6
	ip, iplen := unmap(as_ip6_uint8(a))

	// 4 bytes per column except for the first one (IPFrom)
	colsize := uint32(iplen) + uint32(db.dbcolumn-1)*4

	// row buffer (columns + next IPFrom)
	row := make([]byte, colsize+uint32(iplen))

	// set the initial binary search range
	var off, lower, upper uint32
	if iplen == 4 {
		if off = db.ip4idx; off > 0 {
			off += uint32(ip.lo>>16<<3) - 1
		} else {
			upper = db.ip4count
		}
	} else {
		if off = db.ip6idx; off > 0 {
			off += uint32(ip.hi>>48<<3) - 1
		} else {
			upper = db.ip6count
		}
	}
	if off != 0 {
		// note: len(row) will always be > 8, so we can reuse it here
		if _, err = db.r.ReadAt(row[:8], int64(off)); err != nil {
			return
		}
		lower = as_le_u32(row[0:])
		upper = as_le_u32(row[4:])
	}

	// do the binary search
	for lower <= upper {
		mid := (lower + upper) / 2

		// calculate the current row offset
		if off = mid * colsize; iplen == 4 {
			off += db.ip4base - 1
		} else {
			off += db.ip6base - 1
		}

		// read the row
		if _, err = db.r.ReadAt(row, int64(off)); err != nil {
			return
		}

		// get the row start/end range
		var ipfrom, ipto uint128
		if iplen == 4 {
			ipfrom = as_u32_u128(as_le_u32(row))
			ipto = as_u32_u128(as_le_u32(row[colsize:]))
		} else {
			ipfrom = as_be_u128(row)
			ipto = as_be_u128(row[colsize:])
		}

		// binary search cases
		if ip.Less(ipfrom) {
			upper = mid - 1
			continue
		}
		if ipto == ip || ipto.Less(ip) {
			lower = mid + 1
			continue
		}

		// found
		r.r = db.r
		r.p = db.prcode
		r.t = db.dbtype
		r.d = row[iplen:colsize]
		break
	}
	return
}

// unmap unmaps the v4-mapped or native IPv6 represented by a, returning a raw
// native v4/v6 address and the ip byte length (either 4 or 16).
func unmap(a uint128) (uint128, int) {
	switch {
	case a.hi>>48 == 0x2002:
		// 6to4 -> v4mapped
		a.hi, a.lo = 0, (a.hi>>16)&0xffffffff|0xffff00000000
	case a.hi>>32 == 0x20010000:
		// teredo -> v4mapped
		a.hi, a.lo = 0, (^a.lo)&0xffffffff|0xffff00000000
	}
	if a.hi == 0 && a.lo>>32 == 0xffff {
		// v4mapped -> v4
		a.lo &= 0xffffffff
		return a, 4
	}
	return a, 16
}

// Default options for [Record.String].
var (
	RecordStringColor     = false
	RecordStringMultiline = false
)

// Record points to a database row.
type Record struct {
	r io.ReaderAt
	p DBProduct
	t DBType
	d []byte
}

// IsValid checks whether the record is pointing to a database row.
func (r Record) IsValid() bool {
	return r.d != nil
}

// String gets and formats all fields in the record as a human-readable string.
// Note that this is highly inefficient.
func (r Record) String() string {
	return r.FormatString(RecordStringColor, RecordStringMultiline)
}

// FormatString gets and formats all fields in the record as a human-readable
// string. Note that this is highly inefficient.
func (r Record) FormatString(color, multiline bool) string {
	if !r.IsValid() {
		return ""
	}
	var s strings.Builder
	if color {
		s.WriteString("\x1b[34m")
	}
	s.WriteString(r.p.String())
	if color {
		s.WriteString("\x1b[0m")
	}
	s.WriteByte('<')
	s.WriteString(r.p.FormatType(r.t))
	s.WriteByte('>')
	if color {
		s.WriteString("\x1b[0m")
	}
	if multiline {
		s.WriteString("{\n  ")
	} else {
		s.WriteByte('{')
	}
	for n, f := 0, DBField(1); f < dbFieldUpper; f++ {
		if dt, fd, err := r.get(f); fd.IsValid() { // field exists
			if n++; n > 1 {
				if multiline {
					s.WriteString("\n  ")
				} else {
					s.WriteByte(' ')
				}
			}
			if color {
				s.WriteString("\x1b[35m")
			}
			s.WriteString(f.String())
			if color {
				s.WriteString("\x1b[0m")
			}
			if multiline {
				s.WriteString(" ")
			} else {
				s.WriteByte('=')
			}
			if dt != nil {
				switch fd.Type() {
				case dbft_string:
					if color {
						s.WriteString("\x1b[33m")
					}
					s.WriteString(strconv.Quote(as_strref_unsafe(dt)))
				case dbft_f32le:
					if color {
						s.WriteString("\x1b[32m")
					}
					s.WriteString(strconv.FormatFloat(float64(as_f32(as_le_u32(dt))), 'f', -1, 32))
				}
			} else if err != nil {
				if color {
					s.WriteString("\x1b[31m")
				}
				s.WriteString("<error: ")
				s.WriteString(err.Error())
				s.WriteByte('>')
			}
			if color {
				s.WriteString("\x1b[0m")
			}
		}
	}
	if multiline {
		s.WriteString("\n}")
	} else {
		s.WriteByte('}')
	}
	if color {
		s.WriteString("\x1b[0m")
	}
	return s.String()
}

// MarshalJSON encodes the record as JSON.
func (r Record) MarshalJSON() ([]byte, error) {
	if !r.IsValid() {
		return []byte("null"), nil
	}
	var b bytes.Buffer
	b.WriteByte('{')
	for n, f := 0, DBField(1); f < dbFieldUpper; f++ {
		if dt, fd, err := r.get(f); dt != nil {
			if n++; n > 1 {
				b.WriteByte(',')
			}
			b.WriteByte('"')
			b.WriteString(f.String())
			b.WriteByte('"')
			b.WriteByte(':')
			switch fd.Type() {
			case dbft_string:
				b.WriteString(strconv.Quote(as_strref_unsafe(dt)))
			case dbft_f32le:
				b.WriteString(strconv.FormatFloat(float64(as_f32(as_le_u32(dt))), 'f', -1, 32))
			}
		} else if err != nil {
			return nil, err
		}
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// Get gets f as the default type. If an error occurs or the field is not
// present, nil is returned. This is slightly less efficient than the more
// specific getters.
func (r Record) Get(f DBField) any {
	if dt, fd, _ := r.get(f); dt != nil {
		switch fd.Type() {
		case dbft_string:
			return as_strref_unsafe(dt)
		case dbft_f32le:
			return as_f32(as_le_u32(dt))
		}
	}
	return nil
}

// GetString gets f as a string.
func (r Record) GetString(f DBField) (string, bool) {
	if dt, fd, _ := r.get(f); dt != nil {
		switch fd.Type() {
		case dbft_string:
			return as_strref_unsafe(dt), true
		case dbft_f32le:
			return strconv.FormatFloat(float64(as_f32(as_le_u32(dt))), 'f', -1, 32), true
		}
	}
	return "", false
}

// GetFloat32 gets f as a float32, if possible.
func (r Record) GetFloat32(f DBField) (float32, bool) {
	if dt, fd, _ := r.get(f); dt != nil {
		switch fd.Type() {
		case dbft_string:
			if v, err := strconv.ParseFloat(as_strref_unsafe(dt), 32); err == nil {
				return float32(v), true
			}
		case dbft_f32le:
			return as_f32(as_le_u32(dt)), true
		}
	}
	return 0, false
}

// get gets the raw bytes and field descriptor f in r.
//   - If !r.IsValid or the field does not exist, dt, fd, and err will be zero.
//   - If an error occurs while reading the data, dt will be nil, fd will be
//     valid, and err will be set.
//   - If the read data is too short for the type (most likely due to an
//     unexpected EOF), dt will be nil, fd will be valid, and err will be set.
//   - Otherwise, dt will be set, fd will be valid, and err will be nil.
func (r Record) get(f DBField) (dt []byte, fd dbfd, err error) {
	if !r.IsValid() {
		return
	}

	// get field descriptor
	if fd = dbfds[r.t][r.p][f]; !fd.IsValid() {
		return // no such field
	}

	// get maxfield size
	var sz int
	switch fd.Type() {
	case dbft_string:
		sz = 1 + 0xFF // length byte + max length
	case dbft_f32le:
		sz = 32 / 4
	default:
		panic("unhandled dbft")
	}

	// get column data offset (relative to end of IPFrom column)
	off := (fd.Column() - 2) * 4

	// get field data
	var data []byte
	if ^fd.PtrOffset() == 0 {
		if data = r.d[off:]; len(data) > int(sz) {
			data = data[:sz]
		}
	} else {
		if data = r.d[off:]; len(data) >= 4 {
			b := make([]byte, sz)
			var n int
			if n, err = r.r.ReadAt(b, int64(as_le_u32(data)+uint32(fd.PtrOffset()))); err == nil || err == io.EOF {
				data = b[:n]
			} else {
				return // i/o error
			}
		}
	}

	// parse field data
	if len(data) != 0 {
		switch fd.Type() {
		case dbft_string:
			if len(data) > int(data[0]) {
				dt = data[1 : 1+data[0]]
			}
		case dbft_f32le:
			if len(data) >= int(sz) {
				dt = data
			}
		default:
			panic("unhandled dbft")
		}
	}
	if dt == nil {
		err = io.ErrUnexpectedEOF // too short
	}
	return
}

// as_le_u32 returns the uint32 represented by the little-endian b.
func as_le_u32(b []byte) uint32 {
	_ = b[3] // bounds check hint to compiler; see golang.org/issue/14808
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

// as_le_u64 returns the uint64 represented by the little-endian b.
func as_le_u64(b []byte) uint64 {
	_ = b[7] // bounds check hint to compiler; see golang.org/issue/14808
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
}

// as_f32 returns the float32 represented by u.
func as_f32(u uint32) float32 {
	return *(*float32)(unsafe.Pointer(&u)) // math.Float32frombits
}

// as_strref_unsafe returns b as a string sharing the underlying data.
func as_strref_unsafe(b []byte) string {
	return *(*string)(unsafe.Pointer(&b)) // strings.Builder
}

// as_ip6_uint8 returns a as a uint128 representing a IPv4-mapped or native IPv6.
func as_ip6_uint8(a netip.Addr) uint128 {
	return *(*uint128)(unsafe.Pointer(&a))
}

// as_u32_u128 returns u32 as a uint128.
func as_u32_u128(u32 uint32) uint128 {
	return uint128{lo: uint64(u32)}
}

// as_be_u128 reads a big-endian uint128 from b.
func as_be_u128(b []byte) uint128 {
	_ = b[15] // bounds check hint to compiler; see golang.org/issue/14808
	return uint128{
		hi: as_le_u64(b[8:]),
		lo: as_le_u64(b[0:]),
	}
}

// uint128 represents a uint128 using two uint64s.
type uint128 struct {
	hi uint64
	lo uint64
}

// Less returns true if n < v.
func (n uint128) Less(v uint128) bool {
	return n.hi < v.hi || (n.hi == v.hi && n.lo < v.lo)
}