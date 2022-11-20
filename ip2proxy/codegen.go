//go:build ignore

package main

import "github.com/pg9182/ip2x/internal"

var db = internal.DBInfo{
	Product:     "IP2Proxy",
	ProductCode: 2,
	TypePrefix:  "PX",
	TypeMax:     12,
}

func init() {
	db.StrPtrRel(0, "CountryShort", 2, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3)
	db.StrPtrRel(3, "CountryLong", 2, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3)
	db.StrPtr("Region", 0, 0, 4, 4, 4, 4, 4, 4, 4, 4, 4)
	db.StrPtr("City", 0, 0, 5, 5, 5, 5, 5, 5, 5, 5, 5)
	db.StrPtr("ISP", 0, 0, 0, 6, 6, 6, 6, 6, 6, 6, 6)
	db.StrPtr("ProxyType", 0, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2)
	db.StrPtr("Domain", 0, 0, 0, 0, 7, 7, 7, 7, 7, 7, 7)
	db.StrPtr("UsageType", 0, 0, 0, 0, 0, 8, 8, 8, 8, 8, 8)
	db.StrPtr("ASN", 0, 0, 0, 0, 0, 0, 9, 9, 9, 9, 9)
	db.StrPtr("AS", 0, 0, 0, 0, 0, 0, 10, 10, 10, 10, 10)
	db.StrPtr("LastSeen", 0, 0, 0, 0, 0, 0, 0, 11, 11, 11, 11)
	db.StrPtr("Threat", 0, 0, 0, 0, 0, 0, 0, 0, 12, 12, 12)
	db.StrPtr("Provider", 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 13)

	// https://www.ip2location.com/database/px11-ip-proxytype-country-region-city-isp-domain-usagetype-asn-lastseen-threat-residential-provider @ 2022-11-20
	db.Doc("ProxyType", `Type of proxy.`,
		`  - (VPN) Anonymizing VPN services. These services offer users a publicly accessible VPN for the purpose of hiding their IP address. Anonymity: High.`,
		`  - (TOR) Tor Exit Nodes. The Tor Project is an open network used by those who wish to maintain anonymity. Anonymity: High.`,
		`  - (DCH) Hosting Provider, Data Center or Content Delivery Network. Since hosting providers and data centers can serve to provide anonymity, the Anonymous IP database flags IP addresses associated with them. Anonymity: Low.`,
		`  - (PUB) Public Proxies. These are services which make connection requests on a user's behalf. Proxy server software can be configured by the administrator to listen on some specified port. These differ from VPNs in that the proxies usually have limited functions compare to VPNs. Anonymity: High.`,
		`  - (WEB) Web Proxies. These are web services which make web requests on a user's behalf. These differ from VPNs or Public Proxies in that they are simple web-based proxies rather than operating at the IP address and other ports level. Anonymity: High.`,
		`  - (SES) Search Engine Robots. These are services which perform crawling or scraping to a website, such as, the search engine spider or bots engine. Anonymity: Low.`,
		`  - (RES) Residential proxies. These services offer users proxy connections through residential ISP with or without consents of peers to share their idle resources. Only available with PX10 & PX11. Anonymity: Medium.`)
	db.Doc("CountryShort", `Two-character country code based on ISO 3166.`)
	db.Doc("CountryLong", `Country name based on ISO 3166.`)
	db.Doc("Region", `Region or state name.`)
	db.Doc("City", `City name.`)
	db.Doc("ISP", `Internet Service Provider or company's name.`)
	db.Doc("Domain", `Internet domain name associated with IP address range.`)
	db.Doc("UsageType", `Usage type classification of ISP or company.`,
		`  - (COM) Commercial`,
		`  - (ORG) Organization`,
		`  - (GOV) Government`,
		`  - (MIL) Military`,
		`  - (EDU) University/College/School`,
		`  - (LIB) Library`,
		`  - (CDN) Content Delivery Network`,
		`  - (ISP) Fixed Line ISP`,
		`  - (MOB) Mobile ISP`,
		`  - (DCH) Data Center/Web Hosting/Transit`,
		`  - (SES) Search Engine Spider`,
		`  - (RSV) Reserved`)
	db.Doc("ASN", `Autonomous system number (ASN).`)
	db.Doc("AS", `Autonomous system (AS) name.`)
	db.Doc("LastSeen", `Proxy last seen in days.`)
	db.Doc("Threat", `Security threat reported.`,
		`  - (SPAM) Email and forum spammers`,
		`  - (SCANNER) Network security scanners`,
		`  - (BOTNET) Malware infected devices`)
	db.Doc("Provider", `Name of VPN provider if available.`)
}

func main() {
	internal.DBGenMain(db)
}
