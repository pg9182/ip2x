package test

import (
	"net/netip"
	"testing"
)

func FuzzLookup(f *testing.F) {
	for _, ip := range ips {
		hi, lo := addrUint128(ip)
		f.Add(hi, lo)
	}
	f.Fuzz(func(t *testing.T, hi, lo uint64) {
		var last string
		as := []netip.Addr{
			uint128Addr(hi, lo), // v6 (or v4 if hi is 0)
			uint128Addr(0, lo&0xffffffff|0xffff00000000).Unmap(), // v4
			uint128Addr(0, lo&0xffffffff|0xffff00000000),         //  ^ v4-mapped
			uint128Addr(0x2002<<48|(lo&0xffffffff)<<16, 0),       //  ^ 6to4
			uint128Addr(0x20010000<<32, ^(lo & 0xffffffff)),      //  ^ teredo
		}
		for i, a := range as {
			r, err := IP2x_DB.Lookup(a)
			if err != nil {
				t.Errorf("lookup %s: %v", a, err)
				// not fatal since r should still work on error
			}
			if res := r.Format(false, false); i >= 2 && last != res {
				t.Errorf("lookup %s: expected all v4 mappings to match native v4 (%s): expected %q, got %q", a, as[1], last, res)
			} else {
				last = res
			}
		}
	})
}
