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

var (
	ErrInvalidBin     = errors.New("invalid IP2Location database format (ensure you are using the latest IP2Location BIN file)")
	ErrInvalidAddress = errors.New("invalid IP address")
)

// DBType is an IP2Location database type.
type DBType uint8

// DBTypeMax is the maximum supported DB type
const DBTypeMax DBType = 26

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

// Record is an IP2Location database record.
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

// New initializes a IP2Location database from r.
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
	if db.databaseyear >= 21 && db.productcode != 1 {
		return nil, fmt.Errorf("%w: not an IP2Location database (product code %d)", ErrInvalidBin, db.productcode)
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
		if ipto == addr || ipto.Less(addr) {
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
				case Latitude:
					x.Latitude = math.Float32frombits(binary.LittleEndian.Uint32(row[o:]))
				case Longitude:
					x.Longitude = math.Float32frombits(binary.LittleEndian.Uint32(row[o:]))
				case Domain:
					x.Domain, err = d.readstrptr(row, o, 0)
				case Zipcode:
					x.Zipcode, err = d.readstrptr(row, o, 0)
				case Timezone:
					x.Timezone, err = d.readstrptr(row, o, 0)
				case NetSpeed:
					x.NetSpeed, err = d.readstrptr(row, o, 0)
				case IDDCode:
					x.IDDCode, err = d.readstrptr(row, o, 0)
				case AreaCode:
					x.AreaCode, err = d.readstrptr(row, o, 0)
				case WeatherStationCode:
					x.WeatherStationCode, err = d.readstrptr(row, o, 0)
				case WeatherStationName:
					x.WeatherStationName, err = d.readstrptr(row, o, 0)
				case MCC:
					x.MCC, err = d.readstrptr(row, o, 0)
				case MNC:
					x.MNC, err = d.readstrptr(row, o, 0)
				case MobileBrand:
					x.MobileBrand, err = d.readstrptr(row, o, 0)
				case Elevation:
					var s string
					if s, err = d.readstrptr(row, o, 0); err == nil {
						var v float64
						if v, err = strconv.ParseFloat(s, 32); err == nil {
							x.Elevation = float32(v)
						}
					}
				case UsageType:
					x.UsageType, err = d.readstrptr(row, o, 0)
				case AddressType:
					x.AddressType, err = d.readstrptr(row, o, 0)
				case Category:
					x.Category, err = d.readstrptr(row, o, 0)
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
