package hosts

import (
	"context"
	"errors"
	"net"

	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Hosts is the plugin handler
type Hosts struct {
	*Hostsfile
}

func NewHostsProxy() *Hosts {
	options := newOptions()

	h := &Hosts{
		Hostsfile: &Hostsfile{
			path:    "/etc/hosts",
			hmap:    newHostsMap(),
			options: options,
		},
	}

	go h.readHosts()
	return h
}

// ServeDNS implements the plugin.Handle interface.
func (h Hosts) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()

	answers := []dns.RR{}

	switch state.QType() {
	case dns.TypePTR:
		names := h.LookupStaticAddr(dnsutil.ExtractAddressFromReverse(qname))
		if len(names) == 0 {
			// If this doesn't match we need to fall through regardless of h.Fallthrough
			return dns.RcodeServerFailure, errors.New("no record found")
		}
		answers = h.ptr(qname, h.options.ttl, names)
	case dns.TypeA:
		ips := h.LookupStaticHostV4(qname)
		answers = a(qname, h.options.ttl, ips)
	case dns.TypeAAAA:
		ips := h.LookupStaticHostV6(qname)
		answers = aaaa(qname, h.options.ttl, ips)
	}

	if len(answers) == 0 {

		return dns.RcodeNameError, errors.New("no record found")
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.Answer = answers
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the plugin.Handle interface.
func (h Hosts) Name() string { return "hosts" }

// a takes a slice of net.IPs and returns a slice of A RRs.
func a(zone string, ttl uint32, ips []net.IP) []dns.RR {
	answers := []dns.RR{}
	for _, ip := range ips {
		r := new(dns.A)
		r.Hdr = dns.RR_Header{Name: zone, Rrtype: dns.TypeA,
			Class: dns.ClassINET, Ttl: ttl}
		r.A = ip
		answers = append(answers, r)
	}
	return answers
}

// aaaa takes a slice of net.IPs and returns a slice of AAAA RRs.
func aaaa(zone string, ttl uint32, ips []net.IP) []dns.RR {
	answers := []dns.RR{}
	for _, ip := range ips {
		r := new(dns.AAAA)
		r.Hdr = dns.RR_Header{Name: zone, Rrtype: dns.TypeAAAA,
			Class: dns.ClassINET, Ttl: ttl}
		r.AAAA = ip
		answers = append(answers, r)
	}
	return answers
}

// ptr takes a slice of host names and filters out the ones that aren't in Origins, if specified, and returns a slice of PTR RRs.
func (h *Hosts) ptr(zone string, ttl uint32, names []string) []dns.RR {
	answers := []dns.RR{}
	for _, n := range names {
		r := new(dns.PTR)
		r.Hdr = dns.RR_Header{Name: zone, Rrtype: dns.TypePTR,
			Class: dns.ClassINET, Ttl: ttl}
		r.Ptr = dns.Fqdn(n)
		answers = append(answers, r)
	}
	return answers
}
