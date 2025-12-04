// Command verifier ensures all IPs in an IP2Location database return the same
// information between ip2x and the official libraries.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"
	"os"
	"reflect"
	"time"

	"github.com/ip2location/ip2location-go/v9"
	"github.com/ip2location/ip2proxy-go/v4"
	"github.com/pg9182/ip2x"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s db_file\n", os.Args[0])
		os.Exit(2)
	}

	var r interface {
		io.ReadCloser
		io.ReaderAt
	}
	if buf, err := os.ReadFile(os.Args[1]); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: read database: %v\n", err)
		os.Exit(1)
	} else {
		r = nopCloserAt{bytes.NewReader(buf)}
	}

	db1, err := ip2x.New(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: open database: ip2x: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", db1)

	db2, err := openAdapter(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: open database: official: %v\n", err)
		os.Exit(1)
	}

	var tot int
	fmt.Printf("verifying against %s\nstarting......\n", reflect.TypeOf(db2.DB()).Elem().PkgPath())
	pStart, pLast, pInt, pForce, pClear := time.Now(), time.Now(), time.Millisecond*100, false, "\x1b[A\x1b[K\r"
	if os.Getenv("CI") != "" {
		pInt = time.Second * 5
		pClear = ""
	}
	if err := dbRows(r, func(i, total int, ipfrom, ipto netip.Addr) (err error) {
		defer func() {
			if err != nil {
				err = fmt.Errorf("range [%s, %s): %w", ipfrom, ipto, err)
			}
			if err != nil || pForce || i+1 >= total || time.Since(pLast) > pInt {
				pct := float64(i+1) / float64(total) * 100
				nps := float64(i+1) / time.Since(pStart).Seconds()
				rem := time.Duration(float64(total-i) / nps * float64(time.Second))
				fmt.Printf(pClear+"[%3.0f%%] %.0f rows/sec, %s remaining   [%s, %s)\n", pct, nps, rem.Truncate(time.Second), ipfrom.StringExpanded(), ipto.StringExpanded())
				pLast = time.Now()
				pForce = false
			}
		}()

		tot = total

		rfrom1, err := db1.Lookup(ipfrom)
		if err != nil {
			return fmt.Errorf("lookup %s: ip2x: %v", ipfrom, err)
		}

		rfrom2, err := db2.Lookup(ipfrom)
		if err != nil {
			return fmt.Errorf("lookup %s: official: %v", ipfrom, err)
		}

		if err := dbRecordEquals(rfrom1, rfrom2); err != nil {
			return fmt.Errorf("first (%s) record mismatch (%w):\n\n\tip2x     = %s\n\tofficial = %#v\n\t", ipfrom, err, rfrom1.Format(true, false), rfrom2)
		}

		ipend := ipto.Prev()
		if ipend.IsValid() && ipend.As16()[15] == 0xFF {
			ipend = ipend.Prev()
		}
		if ipend.Compare(ipfrom) <= 0 || !ipend.IsValid() {
			return nil
		}

		rend1, err := db1.Lookup(ipend)
		if err != nil {
			return fmt.Errorf("lookup %s: ip2x: %v", ipend, err)
		}

		rend2, err := db2.Lookup(ipend)
		if err != nil {
			return fmt.Errorf("lookup %s: official: %v", ipend, err)
		}

		if err := dbRecordEquals(rend1, rend2); err != nil {
			if !rend1.IsValid() && dbRecordEmpty(rend2) {
				fmt.Fprintf(os.Stderr, pClear+"note: range [%s, %s): address %s: no record returned by ip2x, but the record from the official library was empty too, so it's probably okay\n\n", ipfrom, ipto, ipend)
				pForce = true
				return nil
			}
			return fmt.Errorf("last (%s) record mismatch (%w):\n\n\tip2x     = %s\n\tofficial = %#v\n\t", ipend, err, rend1.Format(true, false), rend2)
		}

		if err := dbRecordEquals(rend2, rfrom2); err != nil {
			return fmt.Errorf("last official not equal to first (%w) (wtf? does verifier have a bug? or is it the official library?):\n\n\tfirst = %s\n\tlast  = %s\n\t", err, rfrom2, rend2)
		}
		if err := dbRecordEquals(rend1, rfrom1); err != nil {
			return fmt.Errorf("last ip2x not equal to first (%w) (does ip2x have a bug? or is it the official library?):\n\n\tfirst = %s\n\tlast  = %s\n\t", err, rfrom1.Format(true, false), rend1.Format(true, false))
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("ok, %d rows\n", tot)
}

type dbHeader struct {
	DBType   uint8
	DBColumn uint8
	DBYear   uint8
	DBMonth  uint8
	DBDay    uint8
	IP4Count uint32
	IP4Base  uint32
	IP6Count uint32
	IP6Base  uint32
	IP4Idx   uint32
	IP6Idx   uint32
	PRCode   uint8
	PRType   uint8
	FileSize uint32
}

func readDBHeader(r io.ReaderAt) (h dbHeader, err error) {
	if err = binary.Read(io.NewSectionReader(r, 0, 64), binary.LittleEndian, &h); err != nil {
		return
	}
	if h.DBType == 'P' && h.DBColumn == 'K' {
		err = fmt.Errorf("database is zipped")
		return
	}
	if h.DBType == 0 || h.DBMonth == 0 || h.DBMonth > 12 || h.DBDay == 0 || h.DBDay > 31 {
		err = fmt.Errorf("database is corrupt")
		return
	}
	return
}

func dbRecordEmpty(r dbRecordAdapter) bool {
	// HACK: ip2x doesn't include the max field as part of the public API (this
	// won't work if fields are skipped)
	for f := ip2x.DBField(0); f == 0 || f.String() != ""; f++ {
		if v := r.Get(f); v != nil && v != "" && v != "-" && v != float32(0) {
			return false
		}
	}
	return true
}

func dbRecordEquals(act, exp dbRecordAdapter) error {
	// HACK: ip2x doesn't include the max field as part of the public API (this
	// won't work if fields are skipped)
	for f := ip2x.DBField(0); f == 0 || f.String() != ""; f++ {
		if a, e := act.Get(f), exp.Get(f); a != e {
			if a == nil && e == float32(0) {
				return nil
			}
			return fmt.Errorf("%s: expected %#v, got %#v", f, e, a)
		}
	}
	return nil
}

func dbRows(r io.ReaderAt, fn func(i, total int, ipfrom, ipto netip.Addr) error) error {
	h, err := readDBHeader(r)
	if err != nil {
		return err
	}
	var i, total int
	if h.IP4Count > 0 {
		total += int(h.IP4Count) - 1
	}
	if h.IP6Count > 0 {
		total += int(h.IP6Count) - 1
	}
	if h.IP4Count > 0 {
		colsz := 4 + uint32(h.DBColumn-1)*4
		for lower, upper := uint32(0), h.IP4Count-1; lower < upper; lower++ {
			var ipfrom, ipto [4]byte
			if _, err := r.ReadAt(ipfrom[:], int64(h.IP4Base-1+colsz*lower)); err != nil {
				return fmt.Errorf("read IPv4 %d: ipfrom: %w", lower, err)
			}
			if _, err := r.ReadAt(ipto[:], int64(h.IP4Base-1+colsz*lower+colsz)); err != nil {
				return fmt.Errorf("read IPv4 %d: ipto: %w", lower, err)
			}
			for j, n := 0, 4; j < n/2; j++ {
				ipfrom[j], ipfrom[n-j-1] = ipfrom[n-j-1], ipfrom[j]
				ipto[j], ipto[n-j-1] = ipto[n-j-1], ipto[j]
			}
			if err := fn(i, total, netip.AddrFrom4(ipfrom), netip.AddrFrom4(ipto)); err != nil {
				return err
			}
			i++
		}
	}
	if h.IP6Count > 0 {
		colsz := 16 + uint32(h.DBColumn-1)*4
		for lower, upper := uint32(0), h.IP6Count-1; lower < upper; lower++ {
			var ipfrom, ipto [16]byte
			if _, err := r.ReadAt(ipfrom[:], int64(h.IP6Base-1+colsz*lower)); err != nil {
				return fmt.Errorf("read IPv6 %d: ipfrom: %w", lower, err)
			}
			if _, err := r.ReadAt(ipto[:], int64(h.IP6Base-1+colsz*lower+colsz)); err != nil {
				return fmt.Errorf("read IPv6 %d: ipto: %w", lower, err)
			}
			for j, n := 0, 16; j < n/2; j++ {
				ipfrom[j], ipfrom[n-j-1] = ipfrom[n-j-1], ipfrom[j]
				ipto[j], ipto[n-j-1] = ipto[n-j-1], ipto[j]
			}
			if err := fn(i, total, netip.AddrFrom16(ipfrom), netip.AddrFrom16(ipto)); err != nil {
				return err
			}
			i++
		}
	}
	return nil
}

type dbAdapter interface {
	DB() any
	Lookup(netip.Addr) (dbRecordAdapter, error)
}

type dbRecordAdapter interface {
	Get(ip2x.DBField) any
}

func openAdapter(r io.ReaderAt) (dbAdapter, error) {
	h, err := readDBHeader(r)
	if err != nil {
		return nil, err
	}
	switch h.PRCode {
	case 1:
		db, err := ip2location.OpenDBWithReader(nopCloserAt{io.NewSectionReader(r, 0, 1<<63-1)})
		if err != nil {
			return nil, fmt.Errorf("ip2location: %w", err)
		}
		return (*ip2locationAdapter)(db), nil
	case 2:
		db, err := ip2proxy.OpenDBWithReader(nopCloserAt{io.NewSectionReader(r, 0, 1<<63-1)})
		if err != nil {
			return nil, fmt.Errorf("ip2proxy: %w", err)
		}
		return (*ip2proxyAdapter)(db), nil
	}
	return nil, fmt.Errorf("unsupported product code %d", h.PRCode)
}

type ip2locationAdapter ip2location.DB

func (db *ip2locationAdapter) DB() any {
	return (*ip2location.DB)(db)
}

func (db *ip2locationAdapter) Lookup(ip netip.Addr) (dbRecordAdapter, error) {
	r, err := (*ip2location.DB)(db).Get_all(ip.String())
	if err != nil {
		return nil, err
	}
	return ip2locationRecordAdapter(r), nil
}

type ip2proxyAdapter ip2proxy.DB

func (db *ip2proxyAdapter) DB() any {
	return (*ip2proxy.DB)(db)
}

func (db *ip2proxyAdapter) Lookup(ip netip.Addr) (dbRecordAdapter, error) {
	r, err := (*ip2proxy.DB)(db).GetAll(ip.String())
	if err != nil {
		return nil, err
	}
	return ip2proxyRecordAdapter(r), nil
}

type ip2locationRecordAdapter ip2location.IP2Locationrecord

var ip2locationRecordMap = map[ip2x.DBField]string{
	ip2x.CountryCode:        "Country_short",
	ip2x.CountryName:        "Country_long",
	ip2x.Region:             "Region",
	ip2x.City:               "City",
	ip2x.ISP:                "Isp",
	ip2x.Latitude:           "Latitude",
	ip2x.Longitude:          "Longitude",
	ip2x.Domain:             "Domain",
	ip2x.Zipcode:            "Zipcode",
	ip2x.Timezone:           "Timezone",
	ip2x.NetSpeed:           "Netspeed",
	ip2x.IDDCode:            "Iddcode",
	ip2x.AreaCode:           "Areacode",
	ip2x.WeatherStationCode: "Weatherstationcode",
	ip2x.WeatherStationName: "Weatherstationname",
	ip2x.MCC:                "Mcc",
	ip2x.MNC:                "Mnc",
	ip2x.MobileBrand:        "Mobilebrand",
	ip2x.Elevation:          "Elevation",
	ip2x.UsageType:          "Usagetype",
	ip2x.AddressType:        "Addresstype",
	ip2x.Category:           "Category",
	ip2x.District:           "District",
	ip2x.ASN:                "Asn",
	ip2x.AS:                 "As",
	ip2x.ASDomain:           "Asdomain",
	ip2x.ASUsageType:        "Asusagetype",
	ip2x.ASRange:            "Ascidr",
}

func (r ip2locationRecordAdapter) Get(f ip2x.DBField) any {
	const (
		not_supported      = "This parameter is unavailable for selected data file. Please upgrade the data file."
		invalid_address    = "Invalid IP address."
		ipv6_not_supported = "IPv6 address missing in IPv4 BIN."
	)
	if n, ok := ip2locationRecordMap[f]; ok {
		if v := reflect.ValueOf((ip2location.IP2Locationrecord)(r)).FieldByName(n).Interface(); v != not_supported && v != invalid_address && v != ipv6_not_supported {
			return v
		}
	}
	return nil
}

var ip2proxyRecordMap = map[ip2x.DBField]string{
	ip2x.CountryCode: "CountryShort",
	ip2x.CountryName: "CountryLong",
	ip2x.Region:      "Region",
	ip2x.City:        "City",
	ip2x.ISP:         "Isp",
	ip2x.ProxyType:   "ProxyType",
	ip2x.Domain:      "Domain",
	ip2x.UsageType:   "UsageType",
	ip2x.ASN:         "Asn",
	ip2x.AS:          "As",
	ip2x.LastSeen:    "LastSeen",
	ip2x.Threat:      "Threat",
	ip2x.Provider:    "Provider",
	ip2x.FraudScore:  "FraudScore",
}

type ip2proxyRecordAdapter ip2proxy.IP2ProxyRecord

func (r ip2proxyRecordAdapter) Get(f ip2x.DBField) any {
	const (
		msgNotSupported    = "NOT SUPPORTED"
		msgInvalidIP       = "INVALID IP ADDRESS"
		msgIPV6Unsupported = "IPV6 ADDRESS MISSING IN IPV4 BIN"
	)
	if n, ok := ip2proxyRecordMap[f]; ok {
		if v := reflect.ValueOf((ip2proxy.IP2ProxyRecord)(r)).FieldByName(n).Interface(); v != msgNotSupported && v != msgInvalidIP && v != msgIPV6Unsupported {
			return v
		}
	}
	return nil
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
