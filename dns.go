package main

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
)

const SOAString string = "@ SOA prisoner.iana.org. hostmaster.root-servers.org. 2002040800 1800 900 0604800 604800"

func getMockARecord(domain string) dns.RR {
	return &dns.A{
		Hdr: dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 300},
		A:   net.ParseIP("127.0.0.1"),
	}
}

func getMockAAAARecord(domain string) dns.RR {
	return &dns.AAAA{
		Hdr:  dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 300},
		AAAA: net.ParseIP("::1"),
	}
}

func isIPv4(addr net.IP) bool {
	return strings.Contains(addr.String(), ".")
}

func getDomain(domain string) string {
	if dns.IsFqdn(domain) {
		return domain[0 : len(domain)-1]
	} else {
		return domain
	}
}

func getRecords(db Database, fqdn string, rrtype uint16) []dns.RR {
	var records []dns.RR

	ipaddrs, _ := db.GetIPAddresses(nil, getDomain(fqdn))
	switch rrtype {
	case dns.TypeA:
		// FIXME: handle ctx and error
		for _, ip := range ipaddrs {
			if isIPv4(ip) {
				str := fmt.Sprintf("%s 3600 IN A %s", fqdn, ip.String())
				rr, _ := dns.NewRR(str)
				records = append(records, rr)
			}
		}
	case dns.TypeAAAA:
		for _, ip := range ipaddrs {
			if !isIPv4(ip) {
				str := fmt.Sprintf("%s 3600 IN AAAA %s", fqdn, ip.String())
				rr, _ := dns.NewRR(str)
				records = append(records, rr)
			}
		}
	}
	return records
}

func processQuery(db Database, msg *dns.Msg, soa dns.RR, ns []dns.RR) {

	// Multiple questions are never used in practice
	q := msg.Question[0]

	//domainRecords := getDomainRecords(q.Name)
	answer := getRecords(db, q.Name, q.Qtype)

	if len(answer) == 0 {
		// Default response is authoritative with SOA
		msg.Authoritative = true
		msg.Ns = []dns.RR{soa}
		/*
			if len(domainRecords) == 0 {
				// No records for the whole domain
				msg.Rcode = dns.RcodeNameError
			}
		*/
		return
	}

	switch q.Qtype {
	case dns.TypeA, dns.TypeAAAA:
		msg.Authoritative = true
		msg.Ns = ns
		msg.Answer = answer
	}
}

func getHandler(db Database, domain string, nameservers []string) func(dns.ResponseWriter, *dns.Msg) {
	nshdr := dns.RR_Header{Name: domain, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: 3600}

	var nsrr []dns.RR
	for _, ns := range nameservers {
		rr := new(dns.NS)
		rr.Hdr = nshdr
		rr.Ns = dns.Fqdn(ns)
		nsrr = append(nsrr, rr)
	}

	SOAFormat := fmt.Sprintf("%s SOA %s %s %%s 28800 7200 604800 86400", strings.ToLower(domain), strings.ToLower(nameservers[0]), "admin.domain.foo")

	return func(w dns.ResponseWriter, req *dns.Msg) {
		serial := time.Now().Format("2006010215")
		SOAString := fmt.Sprintf(SOAFormat, serial)
		SOA, err := dns.NewRR(SOAString)
		if err != nil {
			// FIXME: Handle error, should not happen
		}

		msg := new(dns.Msg)
		msg.SetReply(req)

		if req.Opcode == dns.OpcodeQuery {
			processQuery(db, msg, SOA, nsrr)
		}
		w.WriteMsg(msg)
	}
}

func startDNS(db Database, config dnsConfig) {
	domain := dns.Fqdn(config.Domain)

	var nsfqdns []string
	for _, nsstr := range config.NameServers {
		nsfqdns = append(nsfqdns, dns.Fqdn(nsstr))
	}

	dns.HandleFunc(domain, getHandler(db, domain, nsfqdns))
	server := &dns.Server{Addr: ":53", Net: "udp"}

	fmt.Printf("Starting DNS server at localhost:53\n")
	log.Fatal(server.ListenAndServe())
}