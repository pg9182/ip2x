package test

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pg9182/ip2x"
)

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
