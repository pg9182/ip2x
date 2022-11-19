// Package ip2proxy reads IP2Proxy databases.
package ip2proxy

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strings"
	"unsafe"
)

var (
	ErrInvalidBin     = errors.New("invalid IP2Location database format (ensure you are using the latest IP2Proxy BIN file)")
	ErrInvalidAddress = errors.New("invalid IP address")
)

// DBType is an IP2Location database type.
type DBType uint8

// DBTypeMax is the maximum supported DB type
const DBTypeMax DBType = 12

// Fields gets the mask of supported fields for the specified DB type.
func (t DBType) Fields() Field {
	var r Field
	if t <= DBTypeMax {
		for f := Field(1); f < All; f <<= 1 {
			if _, ok := f.offset(t); ok {
				r |= f
			}
		}
	}
	return r
}

// offsets gets all offsets for the specified DB type.
func (t DBType) offsets() []uint32 {
	var n int
	for f := Field(1); f < All; f <<= 1 {
		n++
	}
	r, i := make([]uint32, n), 0
	for f := Field(1); f < All; f <<= 1 {
		r[i], _ = f.offset(t)
		i++
	}
	return r
}

// Field is a bitmask representing one or more IP2Location database fields.
type Field uint64

const (
	CountryShort Field = 1 << iota
	CountryLong
	Region
	City
	ISP
	ProxyType
	Domain
	UsageType
	ASN
	AS
	LastSeen
	Threat
	Provider

	// All contains all supported fields.
	All Field = 1<<iota - 1
)

// String returns the name of fields set in f.
func (f Field) String() string {
	var x strings.Builder
	var b bool
	fieldAppendString(&x, &b, f.Has(CountryShort), "CountryShort")
	fieldAppendString(&x, &b, f.Has(CountryLong), "CountryLong")
	fieldAppendString(&x, &b, f.Has(Region), "Region")
	fieldAppendString(&x, &b, f.Has(City), "City")
	fieldAppendString(&x, &b, f.Has(ISP), "ISP")
	fieldAppendString(&x, &b, f.Has(ProxyType), "ProxyType")
	fieldAppendString(&x, &b, f.Has(Domain), "Domain")
	fieldAppendString(&x, &b, f.Has(UsageType), "UsageType")
	fieldAppendString(&x, &b, f.Has(ASN), "ASN")
	fieldAppendString(&x, &b, f.Has(AS), "AS")
	fieldAppendString(&x, &b, f.Has(LastSeen), "LastSeen")
	fieldAppendString(&x, &b, f.Has(Threat), "Threat")
	fieldAppendString(&x, &b, f.Has(Provider), "Provider")
	return x.String()
}

func fieldAppendString(x *strings.Builder, b *bool, c bool, s string) {
	if c {
		if *b {
			x.WriteByte('|')
		} else {
			*b = true
		}
		x.WriteString(s)
	}
}

// Has checks whether all fields in x are in f.
func (f Field) Has(x Field) bool {
	return f&x == x
}

func (f Field) offset(t DBType) (uint32, bool) {
	var v uint8
	switch f {
	case CountryShort, CountryLong:
		v = [DBTypeMax]uint8{0, 2, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}[t]
	case Region:
		v = [DBTypeMax]uint8{0, 0, 0, 4, 4, 4, 4, 4, 4, 4, 4, 4}[t]
	case City:
		v = [DBTypeMax]uint8{0, 0, 0, 5, 5, 5, 5, 5, 5, 5, 5, 5}[t]
	case ISP:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 6, 6, 6, 6, 6, 6, 6, 6}[t]
	case ProxyType:
		v = [DBTypeMax]uint8{0, 0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}[t]
	case Domain:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 7, 7, 7, 7, 7, 7, 7}[t]
	case UsageType:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 8, 8, 8, 8, 8, 8}[t]
	case ASN:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 9, 9, 9, 9, 9}[t]
	case AS:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 10, 10, 10, 10, 10}[t]
	case LastSeen:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 11, 11, 11, 11}[t]
	case Threat:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 12, 12, 12}[t]
	case Provider:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 13}[t]
	default:
		panic("unknown field")
	}
	if v == 0 {
		return 0, false
	}
	return uint32(v-2) << 2, true
}

// Record is an IP2Location database record.
type Record struct {
	Fields       Field
	CountryShort string
	CountryLong  string
	Region       string
	City         string
	ISP          string
	ProxyType    string
	Domain       string
	UsageType    string
	ASN          string
	AS           string
	LastSeen     string
	Threat       string
	Provider     string
}

// IsValid checks whether the record exists in the database.
func (r Record) IsValid() bool {
	return r.Fields != 0
}

// DB is an IP2Location database.
type DB struct {
	r io.ReaderAt

	// cached
	fields  Field
	offsets []uint32

	// header
	databasetype      DBType
	databasecolumn    uint8
	databaseyear      uint8
	databasemonth     uint8
	databaseday       uint8
	ipv4databasecount uint32
	ipv4databaseaddr  uint32
	ipv6databasecount uint32
	ipv6databaseaddr  uint32
	ipv4indexbaseaddr uint32
	ipv6indexbaseaddr uint32
	productcode       uint8
	producttype       uint8
	filesize          uint32
}

// New initializes a IP2Proxy database from r.
func New(r io.ReaderAt) (*DB, error) {
	db := &DB{r: r}

	var row [64]byte // 64-byte header
	if _, err := db.r.ReadAt(row[:], 0); err != nil {
		return nil, err
	}
	db.databasetype = DBType(row[0])
	db.databasecolumn = row[1]
	db.databaseyear = row[2]
	db.databasemonth = row[3]
	db.databaseday = row[4]
	db.ipv4databasecount = binary.LittleEndian.Uint32(row[5:])
	db.ipv4databaseaddr = binary.LittleEndian.Uint32(row[9:])
	db.ipv6databasecount = binary.LittleEndian.Uint32(row[13:])
	db.ipv6databaseaddr = binary.LittleEndian.Uint32(row[17:])
	db.ipv4indexbaseaddr = binary.LittleEndian.Uint32(row[21:])
	db.ipv6indexbaseaddr = binary.LittleEndian.Uint32(row[25:])
	db.productcode = row[29]
	db.producttype = row[30]
	db.filesize = binary.LittleEndian.Uint32(row[31:])

	if db.databasetype == 'P' && db.databasecolumn == 'K' {
		return nil, fmt.Errorf("%w: database is zipped", ErrInvalidBin)
	}
	if db.databaseyear >= 21 && db.productcode != 2 {
		return nil, fmt.Errorf("%w: not an IP2Proxy database (product code %d)", ErrInvalidBin, db.productcode)
	}
	if db.databasetype > DBTypeMax {
		return nil, fmt.Errorf("%w: unsupported db type", ErrInvalidBin)
	}

	db.fields = db.databasetype.Fields()
	db.offsets = db.databasetype.offsets()

	return db, nil
}

// Version returns the database version.
func (d *DB) Version() string {
	return fmt.Sprintf("20%02d-%02d-%02d", d.databaseyear, d.databasemonth, d.databaseday)
}

// Fields returns the supported fields for the database.
func (d *DB) Fields() Field {
	return d.fields
}

// LookupString parses IP and calls Lookup.
func (d *DB) LookupString(ip string) (Record, error) {
	return d.LookupFieldsString(ip, All)
}

// LookupFieldsString parses IP and calls LookupFields.
func (d *DB) LookupFieldsString(ip string, mask Field) (Record, error) {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrInvalidAddress, err)
	}
	return d.LookupFields(a, mask)
}

// Lookup looks up all supported fields for ip.
func (d *DB) Lookup(ip netip.Addr) (Record, error) {
	return d.LookupFields(ip, All)
}

// LookupFields looks up the specified fields for ip. If some fields are
// not supported by the current database type, they will be ignored.
func (d *DB) LookupFields(ip netip.Addr, mask Field) (Record, error) {
	if !ip.IsValid() {
		return Record{}, ErrInvalidAddress
	}

	// limit the fields to the intersection of the database fields and the mask
	mask &= d.fields

	// convert to v4mapped or v6
	/*
		a16 := ip.As16()
		addr := uint128{
			hi: binary.BigEndian.Uint64(a16[:8]),
			lo: binary.BigEndian.Uint64(a16[8:]),
		}
	*/
	addr := *(*uint128)(unsafe.Pointer(&ip)) // so we don't allocate a temporary buffer

	// unmap
	var is4 bool
	switch {
	case addr.hi>>48 == 0x2002:
		// 6to4 -> v4mapped
		addr.hi, addr.lo = 0, (addr.hi>>16)&0xffffffff|0xffff00000000
	case addr.hi>>32 == 0x20010000:
		// teredo -> v4mapped
		addr.hi, addr.lo = 0, (^addr.lo)&0xffffffff|0xffff00000000
	}
	if addr.hi == 0 && addr.lo>>32 == 0xffff {
		// v4mapped -> v4
		addr.lo &= 0xffffffff
		is4 = true
	}

	// calculate the index offset, if present
	var idxoff uint32
	if is4 {
		if d.ipv4indexbaseaddr > 0 {
			idxoff = d.ipv4indexbaseaddr + uint32(addr.lo)>>16<<3
		}
	} else {
		if d.ipv6indexbaseaddr > 0 {
			idxoff = d.ipv6indexbaseaddr + uint32(addr.hi>>48<<3)
		}
	}

	// set the initial binary search range
	var lower, upper uint32
	if idxoff != 0 {
		var row [8]byte
		if _, err := d.r.ReadAt(row[:], int64(idxoff)-1); err != nil {
			return Record{}, err
		}
		lower = binary.LittleEndian.Uint32(row[0:])
		upper = binary.LittleEndian.Uint32(row[4:])
	} else if is4 {
		upper = d.ipv4databasecount
	} else {
		upper = d.ipv6databasecount
	}

	// each row has the ip bytes followed by the fields
	var iplen, colsize uint32
	if is4 {
		iplen = 4
		colsize = uint32(d.databasecolumn * 4) // 4 bytes per column
	} else {
		iplen = 16
		colsize = uint32(d.databasecolumn*4) + 12 // 4 bytes per column, but IPFrom column is 16 bytes
	}
	row := make([]byte, colsize+iplen)

	// do the binary search
	for lower <= upper {
		mid := (lower + upper) / 2

		// calculate the current row offset
		off := mid * colsize
		if is4 {
			off += d.ipv4databaseaddr
		} else {
			off += d.ipv6databaseaddr
		}
		if _, err := d.r.ReadAt(row, int64(off)-1); err != nil {
			return Record{}, err
		}

		// get the row start/end range
		var ipfrom, ipto uint128
		if is4 {
			ipfrom = uint128{lo: uint64(binary.LittleEndian.Uint32(row[0:]))}
			ipto = uint128{lo: uint64(binary.LittleEndian.Uint32(row[colsize:]))}
		} else {
			ipfrom = uint128{
				hi: binary.LittleEndian.Uint64(row[8:]),
				lo: binary.LittleEndian.Uint64(row[0:]),
			}
			ipto = uint128{
				hi: binary.LittleEndian.Uint64(row[colsize+8:]),
				lo: binary.LittleEndian.Uint64(row[colsize:]),
			}
		}

		// binary search cases
		if addr.Less(ipfrom) {
			upper = mid - 1
			continue
		}
		if addr == ipto || ipto.Less(addr) {
			lower = mid + 1
			continue
		}

		// parse the fields
		i, x := 0, Record{
			Fields: mask & d.fields,
		}
		for f := Field(1); f < All; f <<= 1 {
			var err error
			if x.Fields.Has(f) {
				switch o := iplen + d.offsets[i]; f {
				case CountryShort:
					x.CountryShort, err = d.readstrptr(row, o, 0)
				case CountryLong:
					x.CountryLong, err = d.readstrptr(row, o, 3)
				case Region:
					x.Region, err = d.readstrptr(row, o, 0)
				case City:
					x.City, err = d.readstrptr(row, o, 0)
				case ISP:
					x.ISP, err = d.readstrptr(row, o, 0)
				case ProxyType:
					x.ProxyType, err = d.readstrptr(row, o, 0)
				case Domain:
					x.Domain, err = d.readstrptr(row, o, 0)
				case UsageType:
					x.UsageType, err = d.readstrptr(row, o, 0)
				case ASN:
					x.ASN, err = d.readstrptr(row, o, 0)
				case AS:
					x.AS, err = d.readstrptr(row, o, 0)
				case LastSeen:
					x.LastSeen, err = d.readstrptr(row, o, 0)
				case Threat:
					x.Threat, err = d.readstrptr(row, o, 0)
				case Provider:
					x.Provider, err = d.readstrptr(row, o, 0)
				default:
					panic("unimplemented field")
				}
			}
			if err != nil {
				return Record{}, fmt.Errorf("read field %s: %w", f, err)
			}
			i++
		}
		return x, nil
	}

	// no match, so return an empty record
	return Record{}, nil
}

func (d *DB) readstrptr(row []byte, off, rel uint32) (string, error) {
	return d.readstr(binary.LittleEndian.Uint32(row[off:]) + rel)
}

func (d *DB) readstr(pos uint32) (string, error) {
	var data [1 + 0xFF]byte // length byte + max length
	if n, err := d.r.ReadAt(data[:], int64(pos)); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	} else if 1+int(data[0]) >= n {
		return "", fmt.Errorf("string length %d out of range", n)
	}
	return string(data[1 : 1+data[0]]), nil
}

type uint128 struct {
	hi uint64
	lo uint64
}

func (n uint128) Less(v uint128) bool {
	return n.hi < v.hi || (n.hi == v.hi && n.lo < v.lo)
}
