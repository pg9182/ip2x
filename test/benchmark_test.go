package test

import (
	"fmt"
	"os"
	"testing"

	"github.com/ip2location/ip2location-go/v9"
	"github.com/pg9182/ip2x"
)

// go test -run='^$' -bench=.+ -benchmem -count 10 -v . > bench.txt
// go run golang.org/x/perf/cmd/benchstat@latest -row .name -col /lib bench.txt

func TestMain(m *testing.M) {
	fmt.Printf("db: %s\n", IP2x_DB)
	os.Exit(m.Run())
}

func BenchmarkInit(b *testing.B) {
	b.Run("lib=ip2x", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ip2x.New(DB)
		}
	})
	b.Run("lib=ip2location", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ip2location.OpenDBWithReader(DB)
		}
	})
}

func BenchmarkLookupOnly(b *testing.B) {
	b.Run("lib=ip2x", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			IP2x_DB.Lookup(ips[i%len(ips)])
		}
	})
	b.Run("lib=ip2location", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ip2locationv9_query(IP2LocationV9_DB, ipstrs[i%len(ips)], 0)
		}
	})
}

func BenchmarkGetAll(b *testing.B) {
	b.Run("lib=ip2x", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r, _ := IP2x_DB.Lookup(ips[i%len(ips)])
			IP2x_DB.EachField(func(d ip2x.DBField) bool {
				r.Get(d)
				return true
			})
		}
	})
	b.Run("lib=ip2location", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			IP2LocationV9_DB.Get_all(ipstrs[i%len(ips)])
		}
	})
}

func BenchmarkGetOneString(b *testing.B) {
	b.Run("lib=ip2x", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r, _ := IP2x_DB.Lookup(ips[i%len(ips)])
			r.GetString(ip2x.CountryCode)
		}
	})
	b.Run("lib=ip2location", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			IP2LocationV9_DB.Get_country_short(ipstrs[i%len(ips)])
		}
	})
}

func BenchmarkGetOneFloat(b *testing.B) {
	b.Run("lib=ip2x", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r, _ := IP2x_DB.Lookup(ips[i%len(ips)])
			r.GetFloat32(ip2x.Latitude)
		}
	})
	b.Run("lib=ip2location", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			IP2LocationV9_DB.Get_latitude(ipstrs[i%len(ips)])
		}
	})
}

func BenchmarkGetTwoString(b *testing.B) {
	b.Run("lib=ip2x", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r, _ := IP2x_DB.Lookup(ips[i%len(ips)])
			r.GetString(ip2x.CountryCode)
			r.GetString(ip2x.Region)
		}
	})
	b.Run("lib=ip2location", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			IP2LocationV9_DB.Get_country_short(ipstrs[i%len(ips)])
			IP2LocationV9_DB.Get_country_long(ipstrs[i%len(ips)])
		}
	})
}

func BenchmarkGetTwoFloat(b *testing.B) {
	b.Run("lib=ip2x", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r, _ := IP2x_DB.Lookup(ips[i%len(ips)])
			r.GetFloat32(ip2x.Latitude)
			r.GetFloat32(ip2x.Longitude)
		}
	})
	b.Run("lib=ip2location", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			IP2LocationV9_DB.Get_latitude(ipstrs[i%len(ips)])
			IP2LocationV9_DB.Get_longitude(ipstrs[i%len(ips)])
		}
	})
}

func BenchmarkGetNonexistent(b *testing.B) {
	b.Run("lib=ip2x", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			r, _ := IP2x_DB.Lookup(ips[i%len(ips)])
			r.GetString(ip2x.MCC)
		}
	})
	b.Run("lib=ip2location", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			IP2LocationV9_DB.Get_mcc(ipstrs[i%len(ips)])
		}
	})
}
