package doh

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gollum/gollum/cache"
	"github.com/gollum/gollum/utility"
	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"
)

// DNSContent is a rough translation of the Ali DNS over HTTP API as it currently exists.
type DNSContent struct {
	Host  string   `json:"host"`
	Ips   []string `json:"ips"`
	Ipsv6 []string `json:"ipsv6"`
	TTL   int64    `json:"ttl"`
}

// Convert DNSContent to DNSRR
func convertContent(d *DNSContent) []DNSRR {
	rr := make([]DNSRR, 0)

	// get ipv4
	for _, i := range d.Ips {
		var tmp DNSRR
		tmp.Name = d.Host
		tmp.TTL = int32(d.TTL)
		tmp.Type = int32(dns.TypeA)
		tmp.Data = i
		rr = append(rr, tmp)
	}

	// get ipv6
	for _, i := range d.Ipsv6 {
		var tmp DNSRR
		tmp.Name = d.Host
		tmp.TTL = int32(d.TTL)
		tmp.Type = int32(dns.TypeAAAA)
		tmp.Data = i
		rr = append(rr, tmp)
	}
	return rr
}

// AliDoH is data struct for AliCloud dns-over-http
type AliDoH struct {
	// HTTPDnServer is endpoint of google dns
	HTTPDnServer string
}

// GetADoH will init a Ali DOH instance
func GetADoH() *AliDoH {
	tmp := new(AliDoH)
	tmp.HTTPDnServer = "http://203.107.1.33/[UID]/sign_d?host=%s&t=%d&s=%x&query=4,6"
	return tmp
}

func (a *AliDoH) httpDNSRequestProxy(req *dns.Msg) (*dns.Msg, error) {
	l := utility.GetLogger()
	var hClient = &http.Client{
		Timeout: time.Second * 3,
	}

	// AliDoh is query based on signature
	url := utility.GenerateSign(req.Question[0].Name, a.HTTPDnServer)
	r, err := hClient.Get(url)
	if err != nil {
		l.WithFields(log.Fields{
			"request_id": req.Id}).Errorln(err)
		return nil, err
	}

	defer r.Body.Close()

	// Parse the JSON response
	dnsResp := new(DNSContent)
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&dnsResp)
	if err != nil {
		return nil, err
	}

	dnsrr := convertContent(dnsResp)
	// covenrt json to respond
	// Parse the Questions to DNS RRs
	questions := []dns.Question{}
	for idx, c := range req.Question {
		questions = append(questions, dns.Question{
			Name:   c.Name,
			Qtype:  uint16(c.Qtype),
			Qclass: req.Question[idx].Qclass,
		})
	}

	answers := []dns.RR{}
	for _, a := range dnsrr {
		answers = append(answers, newRR(a))
	}

	resp := dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 req.Id,
			Response:           true,
			Opcode:             dns.OpcodeQuery,
			Authoritative:      false,
			Truncated:          false,
			RecursionDesired:   true,
			RecursionAvailable: true,
			//Zero: false,
			AuthenticatedData: false,
			CheckingDisabled:  false,
			Rcode:             0,
		},
		Compress: req.Compress,
		Question: questions,
		Answer:   answers,
	}
	return &resp, nil
}

// AliForward is DNS handler with google httpdns
func (a *AliDoH) AliForward(c *cache.DNSRecorder, r *dns.Msg, HdnsChannel chan *dns.Msg) {
	defer sentry.Recover()
	logger := utility.GetLogger()
	// Google http dns endpoint
	// doh := "https://dns.google.com/resolve"
	rmsg, errm := a.httpDNSRequestProxy(r)
	if errm != nil {
		logger.WithFields(log.Fields{
			"request_id": r.Id}).Errorf("Failed to set up Ali http request due to %s", errm)
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
