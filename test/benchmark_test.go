package test

import (
	"fmt"
	"os"
	"testing"

	"github.com/ip2location/ip2location-go/v9"
	"github.com/pg9182/ip2x"
)

func TestMain(m *testing.M) {
	fmt.Printf("db: %s\n", IP2x_DB)
	os.Exit(m.Run())
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
