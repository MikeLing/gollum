package doh

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gollum/gollum/cache"
	"github.com/gollum/gollum/utility"
	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"
)

// DNSResponseJSON is a rough translation of the Google DNS over HTTP API as it currently exists.
type DNSResponseJSON struct {
	Status           int32         `json:"Status,omitempty"`
	TC               bool          `json:"TC,omitempty"`
	RD               bool          `json:"RD,omitempty"`
	RA               bool          `json:"RA,omitempty"`
	AD               bool          `json:"AD,omitempty"`
	CD               bool          `json:"CD,omitempty"`
	Question         []DNSQuestion `json:"Question,omitempty"`
	Answer           []DNSRR       `json:"Answer,omitempty"`
	Authority        []DNSRR       `json:"Authority,omitempty"`
	Additional       []DNSRR       `json:"Additional,omitempty"`
	EdnsClientSubnet string        `json:"edns_client_subnet,omitempty"`
	Comment          string        `json:"Comment,omitempty"`
}

// DNSQuestion is the JSON encoding of a DNS request
type DNSQuestion struct {
	Name string `json:"name,omitempty"`
	Type int32  `json:"type,omitempty"`
}

// DNSRR is the JSON encoding of an RRset as returned by Google.
type DNSRR struct {
	Name string `json:"name,omitempty"`
	Type int32  `json:"type,omitempty"`
	TTL  int32  `json:"TTL,omitempty"`
	Data string `json:"data,omitempty"`
}

// newRR initializes a new RRGeneric from a DNSRR
func newRR(a DNSRR) dns.RR {
	var rr dns.RR

	// Build an RR header
	rrhdr := dns.RR_Header{
		Name:     a.Name,
		Rrtype:   uint16(a.Type),
		Class:    dns.ClassINET,
		Ttl:      uint32(a.TTL),
		Rdlength: uint16(len(a.Data)),
	}
	constructor, ok := dns.TypeToRR[uint16(a.Type)]
	if ok {
		// Construct a new RR
		rr = constructor()
		*(rr.Header()) = rrhdr
		switch v := rr.(type) {
		case *dns.A:
			v.A = net.ParseIP(a.Data)
		case *dns.AAAA:
			v.AAAA = net.ParseIP(a.Data)
		case *dns.CNAME:
			v.Target = a.Data
		}
	} else {
		// RFC3597 represents an unknown/generic RR.
		// See RFC 3597[http://tools.ietf.org/html/rfc3597].
		rr = dns.RR(&dns.RFC3597{
			Hdr:   rrhdr,
			Rdata: a.Data,
		})
	}
	return rr
}

// parseJsonToDNSMsg is about to parse json respond to dns msg
func parseJsonToDNSMsg(d *DNSResponseJSON, req *dns.Msg) (*dns.Msg, error) {
	// Parse the Questions to DNS RRs
	questions := []dns.Question{}
	for idx, c := range d.Question {
		questions = append(questions, dns.Question{
			Name:   c.Name,
			Qtype:  uint16(c.Type),
			Qclass: req.Question[idx].Qclass,
		})
	}

	// Parse RRs to DNS RRs
	answers := []dns.RR{}
	for _, a := range d.Answer {
		answers = append(answers, newRR(a))
	}

	// Parse RRs to DNS RRs
	authorities := []dns.RR{}
	for _, ns := range d.Authority {
		authorities = append(authorities, newRR(ns))
	}

	// Parse RRs to DNS RRs
	extras := []dns.RR{}
	for _, extra := range d.Additional {
		authorities = append(authorities, newRR(extra))
	}

	resp := dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 req.Id,
			Response:           (d.Status == 0),
			Opcode:             dns.OpcodeQuery,
			Authoritative:      false,
			Truncated:          d.TC,
			RecursionDesired:   d.RD,
			RecursionAvailable: d.RA,
			//Zero: false,
			AuthenticatedData: d.AD,
			CheckingDisabled:  d.CD,
			Rcode:             int(d.Status),
		},
		Compress: req.Compress,
		Question: questions,
		Answer:   answers,
		Ns:       authorities,
		Extra:    extras,
	}

	return &resp, nil
}

// GoogleDoH is data struct for google dns-over-http
type GoogleDoH struct {
	// HTTPDnServer is endpoint of google dns
	HTTPDnServer string
}

// GetGDoH will init a Google DOH instance
func GetGDoH() *GoogleDoH {
	tmp := new(GoogleDoH)
	tmp.HTTPDnServer = "https://dns.google.com/resolve"
	return tmp
}

func (g *GoogleDoH) httpDNSRequestProxy(req *dns.Msg) (*dns.Msg, error) {
	httpreq, err := http.NewRequest(http.MethodGet, g.HTTPDnServer, nil)
	if err != nil {
		return nil, err
	}

	qry := httpreq.URL.Query()
	qry.Add("name", req.Question[0].Name)
	qry.Add("type", fmt.Sprintf("%v", req.Question[0].Qtype))

	httpreq.URL.RawQuery = qry.Encode()

	httpresp, err := http.DefaultClient.Do(httpreq)
	if err != nil {
		return nil, err
	}
	defer httpresp.Body.Close()

	// Parse the JSON response
	dnsResp := new(DNSResponseJSON)
	decoder := json.NewDecoder(httpresp.Body)
	err = decoder.Decode(&dnsResp)
	if err != nil {
		return nil, err
	}

	// covenrt json to dnsmsg and return
	return parseJsonToDNSMsg(dnsResp, req)
}

// GForward is DNS handler with google httpdns
func (g *GoogleDoH) GForward(c *cache.DNSRecorder, r *dns.Msg, HdnsChannel chan *dns.Msg) {
	logger := utility.GetLogger()
	// Google http dns endpoint
	// doh := "https://dns.google.com/resolve"

	defer sentry.Recover()
	rmsg, errm := g.httpDNSRequestProxy(r)
	if errm != nil {
		logger.WithFields(log.Fields{
			"request_id": r.Id}).Errorf("Failed to set up google http request due to %s", errm)
		return
	}
	// Add RR into cache
	if len(rmsg.Answer) > 0 {
		t := rmsg.Answer[0].Header().Ttl
		e := time.Duration(t+10) * time.Second
		c.Set(rmsg.Answer, e, r.Question[0].Qtype)
		select {
		case HdnsChannel <- rmsg:
		default:
		}
	} else {
		logger.WithFields(log.Fields{
			"request_id": r.Id}).Errorf("No answers has been returned as %s", rmsg)
	}

	return
}
