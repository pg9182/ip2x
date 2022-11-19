// Package ip2location reads IP2Location databases.
package ip2location

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"unsafe"
)

const (
	DBProduct     = "IP2Location"
	DBProductCode = 1
	DBTypePrefix  = "DB"
	DBTypeMax     = DBType(26)
)

var (
	ErrInvalidBin     = errors.New("invalid database format (ensure you are using the latest " + DBProduct + " BIN file)")
	ErrInvalidAddress = errors.New("invalid IP address")
)

// DBType is the database type.
type DBType uint8

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

// Field is a bitmask representing one or more database fields.
type Field uint64

const (
	CountryShort Field = 1 << iota
	CountryLong
	Region
	City
	ISP
	Latitude
	Longitude
	Domain
	Zipcode
	Timezone
	NetSpeed
	IDDCode
	AreaCode
	WeatherStationCode
	WeatherStationName
	MCC
	MNC
	MobileBrand
	Elevation
	UsageType
	AddressType
	Category

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
	fieldAppendString(&x, &b, f.Has(Latitude), "Latitude")
	fieldAppendString(&x, &b, f.Has(Longitude), "Longitude")
	fieldAppendString(&x, &b, f.Has(Domain), "Domain")
	fieldAppendString(&x, &b, f.Has(Zipcode), "Zipcode")
	fieldAppendString(&x, &b, f.Has(Timezone), "Timezone")
	fieldAppendString(&x, &b, f.Has(NetSpeed), "NetSpeed")
	fieldAppendString(&x, &b, f.Has(IDDCode), "IDDCode")
	fieldAppendString(&x, &b, f.Has(AreaCode), "AreaCode")
	fieldAppendString(&x, &b, f.Has(WeatherStationCode), "WeatherStationCode")
	fieldAppendString(&x, &b, f.Has(WeatherStationName), "WeatherStationName")
	fieldAppendString(&x, &b, f.Has(MCC), "MCC")
	fieldAppendString(&x, &b, f.Has(MNC), "MNC")
	fieldAppendString(&x, &b, f.Has(MobileBrand), "MobileBrand")
	fieldAppendString(&x, &b, f.Has(Elevation), "Elevation")
	fieldAppendString(&x, &b, f.Has(UsageType), "UsageType")
	fieldAppendString(&x, &b, f.Has(AddressType), "AddressType")
	fieldAppendString(&x, &b, f.Has(Category), "Category")
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
		v = [DBTypeMax]uint8{0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2}[t]
	case Region:
		v = [DBTypeMax]uint8{0, 0, 0, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3}[t]
	case City:
		v = [DBTypeMax]uint8{0, 0, 0, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4}[t]
	case ISP:
		v = [DBTypeMax]uint8{0, 0, 3, 0, 5, 0, 7, 5, 7, 0, 8, 0, 9, 0, 9, 0, 9, 0, 9, 7, 9, 0, 9, 7, 9, 9}[t]
	case Latitude:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 5, 5, 0, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5, 5}[t]
	case Longitude:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 6, 6, 0, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6, 6}[t]
	case Domain:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 6, 8, 0, 9, 0, 10, 0, 10, 0, 10, 0, 10, 8, 10, 0, 10, 8, 10, 10}[t]
	case Zipcode:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 7, 7, 7, 7, 0, 7, 7, 7, 0, 7, 0, 7, 7, 7, 0, 7, 7}[t]
	case Timezone:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 8, 8, 7, 8, 8, 8, 7, 8, 0, 8, 8, 8, 0, 8, 8}[t]
	case NetSpeed:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 8, 11, 0, 11, 8, 11, 0, 11, 0, 11, 0, 11, 11}[t]
	case IDDCode:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9, 12, 0, 12, 0, 12, 9, 12, 0, 12, 12}[t]
	case AreaCode:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 13, 0, 13, 0, 13, 10, 13, 0, 13, 13}[t]
	case WeatherStationCode:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9, 14, 0, 14, 0, 14, 0, 14, 14}[t]
	case WeatherStationName:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 15, 0, 15, 0, 15, 0, 15, 15}[t]
	case MCC:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 9, 16, 0, 16, 9, 16, 16}[t]
	case MNC:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 10, 17, 0, 17, 10, 17, 17}[t]
	case MobileBrand:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 11, 18, 0, 18, 11, 18, 18}[t]
	case Elevation:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 11, 19, 0, 19, 19}[t]
	case UsageType:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 12, 20, 20}[t]
	case AddressType:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 21}[t]
	case Category:
		v = [DBTypeMax]uint8{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 22}[t]
	default:
		panic("unknown field")
	}
	if v == 0 {
		return 0, false
	}
	return uint32(v-2) << 2, true
}

// Record contains information about an IP.
type Record struct {
	Fields             Field
	CountryShort       string
	CountryLong        string
	Region             string
	City               string
	ISP                string
	Latitude           float32
	Longitude          float32
	Domain             string
	Zipcode            string
	Timezone           string
	NetSpeed           string
	IDDCode            string
	AreaCode           string
	WeatherStationCode string
	WeatherStationName string
	MCC                string
	MNC                string
	MobileBrand        string
	Elevation          float32
	UsageType          string
	AddressType        string
	Category           string
}

// IsValid checks whether the record exists in the database.
func (r Record) IsValid() bool {
	return r.Fields != 0
}

// DB efficiently reads an IP database.
type DB struct {
	r io.ReaderAt

	fld Field
	off []uint32
	hdr dbheader
}

type dbheader struct {
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

// New initializes a database from r.
func New(r io.ReaderAt) (*DB, error) {
	db := &DB{r: r}

	var row [64]byte // 64-byte header
	if _, err := db.r.ReadAt(row[:], 0); err != nil {
		return nil, err
	}
	db.hdr.databasetype = DBType(row[0])
	db.hdr.databasecolumn = row[1]
	db.hdr.databaseyear = row[2]
	db.hdr.databasemonth = row[3]
	db.hdr.databaseday = row[4]
	db.hdr.ipv4databasecount = binary.LittleEndian.Uint32(row[5:])
	db.hdr.ipv4databaseaddr = binary.LittleEndian.Uint32(row[9:])
	db.hdr.ipv6databasecount = binary.LittleEndian.Uint32(row[13:])
	db.hdr.ipv6databaseaddr = binary.LittleEndian.Uint32(row[17:])
	db.hdr.ipv4indexbaseaddr = binary.LittleEndian.Uint32(row[21:])
	db.hdr.ipv6indexbaseaddr = binary.LittleEndian.Uint32(row[25:])
	db.hdr.productcode = row[29]
	db.hdr.producttype = row[30]
	db.hdr.filesize = binary.LittleEndian.Uint32(row[31:])

	if db.hdr.databasetype == 'P' && db.hdr.databasecolumn == 'K' {
		return nil, fmt.Errorf("%w: database is zipped", ErrInvalidBin)
	}
	if db.hdr.databaseyear >= 21 && db.hdr.productcode != DBProductCode {
		return nil, fmt.Errorf("%w: not an %s database (product code %d)", ErrInvalidBin, DBProduct, db.hdr.productcode)
	}
	if db.hdr.databasetype > DBTypeMax {
		return nil, fmt.Errorf("%w: unsupported db type", ErrInvalidBin)
	}

	db.fld = db.hdr.databasetype.Fields()
	db.off = db.hdr.databasetype.offsets()

	return db, nil
}

// String returns a human-readable string describing the database.
func (d *DB) String() string {
	var ipv string
	if v4, v6 := d.HasIPv4(), d.HasIPv6(); v4 && !v6 {
		ipv = "IPv4"
	} else if !v4 && v6 {
		ipv = "IPv6"
	} else {
		ipv = "IPv4+IPv6"
	}
	return DBProduct + " " + d.Version() + " " + DBTypePrefix + strconv.Itoa(int(d.hdr.databasetype)) + " [" + d.Fields().String() + "]" + " (" + ipv + ")"
}

// Version returns the database version.
func (d *DB) Version() string {
	return fmt.Sprintf("20%02d-%02d-%02d", d.hdr.databaseyear, d.hdr.databasemonth, d.hdr.databaseday)
}

// HasIPv4 returns true if the database contains IPv4 entries.
func (d *DB) HasIPv4() bool {
	return d.hdr.ipv4databasecount != 0
}

// HasIPv6 returns true if the database contains HasIPv6 entries.
func (d *DB) HasIPv6() bool {
	return d.hdr.ipv6databasecount != 0
}

// Fields returns the supported fields for the database.
func (d *DB) Fields() Field {
	return d.fld
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
	// unmap the ip address into a native v4/v6
	addr, is4, err := unmap(ip)
	if err != nil {
		return Record{}, err
	}

	// set the initial binary search range
	lower, upper, err := d.index(addr, is4)
	if err != nil {
		return Record{}, err
	}

	// each row has the ip bytes followed by the fields
	var iplen uint32
	if is4 {
		iplen = 4
	} else {
		iplen = 16
	}

	// 4 bytes per column except for the first one (IPFrom)
	colsize := iplen + uint32(d.hdr.databasecolumn-1)*4

	// do the binary search
	row := make([]byte, colsize+iplen)
	for lower <= upper {
		mid := (lower + upper) / 2

		// calculate the current row offset
		off := mid * colsize
		if is4 {
			off += d.hdr.ipv4databaseaddr
		} else {
			off += d.hdr.ipv6databaseaddr
		}

		// read the row
		if _, err := d.r.ReadAt(row, int64(off)-1); err != nil {
			return Record{}, err
		}

		// get the row start/end range
		var ipfrom, ipto uint128
		if is4 {
			ipfrom = u128(binary.LittleEndian.Uint32(row))
			ipto = u128(binary.LittleEndian.Uint32(row[colsize:]))
		} else {
			ipfrom = beUint128(row)
			ipto = beUint128(row[colsize:])
		}

		// binary search cases
		if addr.Less(ipfrom) {
			upper = mid - 1
			continue
		}
		if ipto == addr || ipto.Less(addr) {
			lower = mid + 1
			continue
		}
		return d.record(row[iplen:], mask)
	}

	// no match, so return an empty record
	return Record{}, nil
}

// record decodes the fields specified by mask from row.
func (d *DB) record(rowdata []byte, mask Field) (Record, error) {
	i, x := 0, Record{
		Fields: mask & d.fld,
	}
	for f := Field(1); f < All; f <<= 1 {
		var err error
		if x.Fields.Has(f) {
			switch f {
			case CountryShort:
				x.CountryShort, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case CountryLong:
				x.CountryLong, err = readstrptr(d.r, rowdata, d.off[i], 3)
			case Region:
				x.Region, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case City:
				x.City, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case ISP:
				x.ISP, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case Latitude:
				x.Latitude = math.Float32frombits(binary.LittleEndian.Uint32(rowdata[d.off[i]:]))
			case Longitude:
				x.Longitude = math.Float32frombits(binary.LittleEndian.Uint32(rowdata[d.off[i]:]))
			case Domain:
				x.Domain, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case Zipcode:
				x.Zipcode, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case Timezone:
				x.Timezone, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case NetSpeed:
				x.NetSpeed, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case IDDCode:
				x.IDDCode, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case AreaCode:
				x.AreaCode, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case WeatherStationCode:
				x.WeatherStationCode, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case WeatherStationName:
				x.WeatherStationName, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case MCC:
				x.MCC, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case MNC:
				x.MNC, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case MobileBrand:
				x.MobileBrand, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case Elevation:
				var s string
				if s, err = readstrptr(d.r, rowdata, d.off[i], 0); err == nil {
					var v float64
					if v, err = strconv.ParseFloat(s, 32); err == nil {
						x.Elevation = float32(v)
					}
				}
			case UsageType:
				x.UsageType, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case AddressType:
				x.AddressType, err = readstrptr(d.r, rowdata, d.off[i], 0)
			case Category:
				x.Category, err = readstrptr(d.r, rowdata, d.off[i], 0)
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

// index determines the lower and upper search offset for a, using the index if
// present.
func (d *DB) index(a uint128, is4 bool) (lower, upper uint32, err error) {
	var idxoff uint32
	if is4 {
		if d.hdr.ipv4indexbaseaddr > 0 {
			idxoff = d.hdr.ipv4indexbaseaddr + uint32(a.lo)>>16<<3
		}
	} else {
		if d.hdr.ipv6indexbaseaddr > 0 {
			idxoff = d.hdr.ipv6indexbaseaddr + uint32(a.hi>>48<<3)
		}
	}
	if idxoff == 0 {
		if is4 {
			upper = d.hdr.ipv4databasecount
		} else {
			upper = d.hdr.ipv6databasecount
		}
		return
	}
	var row [8]byte
	if _, err = d.r.ReadAt(row[:], int64(idxoff)-1); err == nil {
		lower = binary.LittleEndian.Uint32(row[0:])
		upper = binary.LittleEndian.Uint32(row[4:])
	}
	return
}

// readstrptr reads the string from r at *(*(row + off) + rel).
func readstrptr(r io.ReaderAt, row []byte, off, rel uint32) (string, error) {
	off = binary.LittleEndian.Uint32(row[off:]) + rel

	var data [1 + 0xFF]byte // length byte + max length
	if n, err := r.ReadAt(data[:], int64(off)); err != nil && !errors.Is(err, io.EOF) {
		return "", err
	} else if 1+int(data[0]) >= n {
		return "", fmt.Errorf("string length %d out of range", n)
	}
	return string(data[1 : 1+data[0]]), nil
}

// uint128 represents a uint128 using two uint64s.
type uint128 struct {
	hi uint64
	lo uint64
}

// u128 returns a uint32 as a uint128.
func u128(u32 uint32) uint128 {
	return uint128{lo: uint64(u32)}
}

// beUint128 reads a big-endian uint128 from b.
func beUint128(b []byte) uint128 {
	_ = b[15] // bounds check hint to compiler; see golang.org/issue/14808
	return uint128{
		hi: binary.LittleEndian.Uint64(b[8:]),
		lo: binary.LittleEndian.Uint64(b[0:]),
	}
}

// Less returns true if n < v.
func (n uint128) Less(v uint128) bool {
	return n.hi < v.hi || (n.hi == v.hi && n.lo < v.lo)
}

// as16 returns a as a IPv4-mapped or native IPv6.
func as16(a netip.Addr) uint128 {
	/*
		a16 := a.As16()
		return uint128{
			hi: binary.BigEndian.Uint64(a16[:8]),
			lo: binary.BigEndian.Uint64(a16[8:]),
		}
	*/
	return *(*uint128)(unsafe.Pointer(&a))
}

// unmap unmaps a, returning a raw v4/v6 address and whether it is an IPv4.
func unmap(a netip.Addr) (uint128, bool, error) {
	if !a.IsValid() {
		return uint128{}, false, ErrInvalidAddress
	}
	r := as16(a)

	switch {
	case r.hi>>48 == 0x2002:
		// 6to4 -> v4mapped
		r.hi, r.lo = 0, (r.hi>>16)&0xffffffff|0xffff00000000
	case r.hi>>32 == 0x20010000:
		// teredo -> v4mapped
		r.hi, r.lo = 0, (^r.lo)&0xffffffff|0xffff00000000
	}

	if r.hi == 0 && r.lo>>32 == 0xffff {
		// v4mapped -> v4
		r.lo &= 0xffffffff
		return r, true, nil
	}
	return r, false, nil
}
