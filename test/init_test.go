package test

import (
	"bytes"
	"encoding/binary"
	"io"
	"net/netip"
	"os"
	_ "unsafe"

	"github.com/ip2location/ip2location-go/v9"
	"github.com/pg9182/ip2x"
)

//go:linkname ip2locationv9_query github.com/ip2location/ip2location-go/v9.(*DB).query
func ip2locationv9_query(db *ip2location.DB, ipaddress string, mode uint32) (ip2location.IP2Locationrecord, error)

const (
	ip2locationv9_invalid_address = "Invalid IP address."
	ip2locationv9_not_supported   = "This parameter is unavailable for selected data file. Please upgrade the data file."
)

var (
	DB interface {
		io.ReadCloser
		io.ReaderAt
	}
	IP2LocationV9_DB *ip2location.DB
	IP2x_DB          *ip2x.DB
)

func init() {
	if ip2locationv9_invalid_address == "" || ip2locationv9_not_supported == "" {
		panic("wtf")
	}

	// note: we read the entire file into memory so we aren't affected by OS disk
	// caching

	// note: currently, both ip2x and the official library will result in the same
	// number of disk reads (one for the index, and one for each row of the binary
	// search, and one for each pointer field being read), so this won't skew the
	// results

	if buf, err := os.ReadFile("IP2LOCATION-LITE-DB11.IPV6.BIN"); err != nil {
		panic(err)
	} else {
		DB = nopCloserAt{bytes.NewReader(buf)}
	}

	// open the databases

	var err error
	if IP2LocationV9_DB, err = ip2location.OpenDBWithReader(DB); err != nil {
		panic(err)
	} else if IP2x_DB, err = ip2x.New(DB); err != nil {
		panic(err)
	}

	// check some things we depend on in the tests

	if !IP2x_DB.HasIPv4() {
		panic("db must support ipv4")
	}
	if !IP2x_DB.HasIPv6() {
		panic("db must support ipv6")
	}
	if !IP2x_DB.Has(ip2x.CountryCode) {
		panic("db must have country code")
	}
	if !IP2x_DB.Has(ip2x.Latitude) {
		panic("db must have latitude")
	}
	if IP2x_DB.Has(ip2x.MCC) {
		panic("db must not have mcc")
	}
}

// a balanced variety of IP addresses for testing.
var ips, ipstrs = mkips(
	"1.2.3.4",
	"5.6.7.8",
	"9.10.11.12",
	"13.14.15.16",
	"123.123.123.123",
	"127.0.0.1",

	// some public dns servers
	"9.9.9.9",
	"8.8.8.8",
	"8.8.4.4",
	"1.1.1.1",
	"208.67.222.222",
	"2620:119:35::35",

	// google.com
	"142.251.41.78",
	"2607:f8b0:400b:803::200e",

	// example.com
	"93.184.216.34",
	"2606:2800:220:1:248:1893:25c8:1946",
)

func mkips(ip ...string) (a []netip.Addr, s []string) {
	for _, x := range ip {
		p := netip.MustParseAddr(x)
		a = append(a, p)

		if !p.Is4() {
			continue
		}

		hi, lo := addrUint128(p)

		// as v4-mapped
		a = append(a, uint128Addr(hi, lo))

		// as 6to4
		a = append(a, uint128Addr(
			0x2002<<48|(lo&0xffffffff)<<16,
			0,
		))

		// as teredo
		a = append(a, uint128Addr(
			0x20010000<<32,
			^(lo&0xffffffff),
		))
	}
	for _, x := range a {
		s = append(s, x.String())
	}
	return
}

func addrUint128(a netip.Addr) (hi, lo uint64) {
	b := a.As16()
	hi = binary.BigEndian.Uint64(b[:8])
	lo = binary.BigEndian.Uint64(b[8:])
	return
}

func uint128Addr(hi, lo uint64) (a netip.Addr) {
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], hi)
	binary.BigEndian.PutUint64(b[8:], lo)
	return netip.AddrFrom16(b)
}

type nopCloserAt struct {
	R interface {
		io.Reader
		io.ReaderAt
	}
}

func (x nopCloserAt) Read(p []byte) (n int, err error) {
	return x.R.Read(p)
}

func (x nopCloserAt) ReadAt(p []byte, off int64) (n int, err error) {
	return x.R.ReadAt(p, off)
}

func (x nopCloserAt) Close() error {
	return nil
}
