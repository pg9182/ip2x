package bench

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/netip"
	"os"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

func TestMain(m *testing.M) {
	fmt.Printf("db: %s\n", IP2x_DB)
	os.Exit(m.Run())
}

func TestCorrectness(t *testing.T) {
	for _, a := range ips {
		t.Run(a.String(), func(t *testing.T) {
			if err := testCorrectness(a); err != nil {
				t.Fatal(err.Error())
			}
		})
	}
}

func TestCorrectnessIPv425bit(t *testing.T) {
	testCorrectnessIPv4Bits(t, 25)
}

func testCorrectnessIPv4Bits(t *testing.T, bits int) {
	if bits < 1 || bits > 32 {
		panic("invalid bits")
	}

	upper := uint32(1)<<uint32(bits) - 1
	t.Logf("bits: %d (%d addresses)", bits, bits)

	const maxErrors = 100
	var numErrors atomic.Uint64

	var wg sync.WaitGroup
	threads := runtime.NumCPU()

	t.Logf("threads: %d", threads)

	const progressBits = 5 // emit progress 2^progressBits times
	var done atomic.Uint32

	start := time.Now()
	for thread := 0; thread < threads; thread++ {
		thread := thread
		wg.Add(1)
		go func() {
			defer wg.Done()
			runtime.LockOSThread()

			for i, u := uint32(0), uint32(1)<<uint32(bits)-1; i <= upper; i++ {
				if numErrors.Load() >= maxErrors {
					return
				}
				if i%uint32(threads) != uint32(thread) {
					continue
				}

				n := i << (32 - bits)
				if done := done.Add(1); n<<progressBits == 0 {
					pct := 100 * float64(i>>(bits-progressBits)) / float64((uint32(1)<<progressBits - 1))
					rem := u - i
					if i == 0 {
						t.Logf("[%3.0f%%] %d remaining", pct, rem)
					} else {
						nps := float64(done) / time.Since(start).Seconds()
						npt := time.Since(start) / time.Duration(done) * time.Duration(rem)
						t.Logf("[%3.0f%%] %d remaining, %s (%.0f/sec)", pct, rem, npt, nps)
					}
				}
				if bits < 32 {
					n += 1 // so we aren't just .0 IPs
				}

				var b [4]byte
				binary.BigEndian.PutUint32(b[:], n)

				a := netip.AddrFrom4(b)
				if err := testCorrectness(a); err != nil {
					t.Errorf("%s: %v", a, err)
					numErrors.Add(1)
				}
			}
		}()
	}

	wg.Wait()
	if numErrors.Load() >= maxErrors {
		t.Fatalf("too many errors")
	}
}

func testCorrectness(a netip.Addr) error {
	fmap := map[ip2x.DBField]string{ // map[ip2x.DBField]ip2location.Record.*
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
	}

	r1, err := IP2LocationV9_DB.Get_all(a.String())
	if err != nil {
		return fmt.Errorf("ip2location/v9 lookup error: %w", err)
	}
	if r1.Country_short == ip2locationv9_invalid_address {
		return fmt.Errorf("ip2location/v9 thinks a valid address is invalid...")
	}

	r2, err := IP2x_DB.Lookup(a)
	if err != nil {
		return fmt.Errorf("ip2x lookup error: %w", err)
	}

	for x, y := range fmap {
		switch v := reflect.ValueOf(r1).FieldByName(y).Interface().(type) {
		case string:
			if exp, act := v != ip2locationv9_not_supported, IP2x_DB.Has(x); exp != act {
				return fmt.Errorf("ip2location/v9 thinks field %s (ip2x.%s) should exist=%t (value %q), but ip2x thinks %t", y, x, exp, v, act)
			}
			if act, _ := r2.GetString(x); v != act && !(v == ip2locationv9_not_supported && act == "") {
				return fmt.Errorf("ip2location/v9 thinks field %s (ip2x.%s) should be %#v, but ip2x thinks %#v", y, x, v, act)
			}
		case float32:
			if act, _ := r2.GetFloat32(x); v != act {
				return fmt.Errorf("ip2location/v9 thinks field %s (ip2x.%s) should be %#v, but ip2x thinks %#v", y, x, v, act)
			}
		case nil:
			panic("invalid ip2location field name " + y + " for mapping of ip2x." + x.GoString())
		default:
			panic("unhandled ip2location type " + reflect.ValueOf(v).Type().String())
		}
	}
	return nil
}

func BenchmarkIP2x_Init(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ip2x.New(DB)
	}
}

func BenchmarkIP2x_LookupOnly(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IP2x_DB.Lookup(ips[i%len(ips)])
	}
}

func BenchmarkIP2x_GetAll(b *testing.B) {
	for i := 0; i < b.N; i++ {
		r, _ := IP2x_DB.Lookup(ips[i%len(ips)])
		IP2x_DB.EachField(func(d ip2x.DBField) bool {
			r.Get(d)
			return true
		})
	}
}

func BenchmarkIP2x_GetOneString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		r, _ := IP2x_DB.Lookup(ips[i%len(ips)])
		r.GetString(ip2x.CountryCode)
	}
}

func BenchmarkIP2x_GetOneFloat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		r, _ := IP2x_DB.Lookup(ips[i%len(ips)])
		r.GetFloat32(ip2x.Latitude)
	}
}

func BenchmarkIP2x_GetNonexistent(b *testing.B) {
	for i := 0; i < b.N; i++ {
		r, _ := IP2x_DB.Lookup(ips[i%len(ips)])
		r.GetString(ip2x.MCC)
	}
}

func BenchmarkIP2LocationV9_Init(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ip2location.OpenDBWithReader(DB)
	}
}

func BenchmarkIP2LocationV9_LookupOnly(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ip2locationv9_query(IP2LocationV9_DB, ipstrs[i%len(ips)], 0)
	}
}

func BenchmarkIP2LocationV9_GetAll(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IP2LocationV9_DB.Get_all(ipstrs[i%len(ips)])
	}
}

func BenchmarkIP2LocationV9_GetOneString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IP2LocationV9_DB.Get_country_short(ipstrs[i%len(ips)])
	}
}

func BenchmarkIP2LocationV9_GetOneFloat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IP2LocationV9_DB.Get_latitude(ipstrs[i%len(ips)])
	}
}

func BenchmarkIP2LocationV9_GetNonexistent(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IP2LocationV9_DB.Get_mcc(ipstrs[i%len(ips)])
	}
}

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
