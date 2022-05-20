/*
dns_handler contains main logic of the gollum dns handler,
it will be called when the dns request is received and returned dns content as UDP packet if necessary.
*/
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gollum/gollum/utility"
	"github.com/miekg/dns"
	"github.com/mohae/deepcopy"
	log "github.com/sirupsen/logrus"
)

// univForwarder is an implementation for None A type DNS request forwarder which will forward request to upstream DNS resolover.
// Basically speaking, it just forwards the dns request to normal DNS resolover with UDP request and returns what it get.
// You don't need to care this function if you only care about http/https DNS requests && response with type A/AAAA.
func univForwarder(w dns.ResponseWriter, r *dns.Msg, EdnsChannel chan *dns.Msg) {
	// otherwise, send request to our predefined upstream solver
	CoreDNSForward.ServeDNS(context.TODO(), w, r, EdnsChannel)
	return
}

// HandleRoot used to handle DNS request.
func HandleRoot(w dns.ResponseWriter, r *dns.Msg) {
	defer sentry.Recover()
	// Set a timer for wait HTTP DNS respond timeout
	l := utility.GetLogger()
	c := utility.GetConfig()
	l.WithFields(log.Fields{
		"request_id": r.Id}).Infoln(r)

	// Try Hosts first
	_, err := GollumHosts.ServeDNS(context.TODO(), w, r)
	if err == nil {
		StatsD.Increment("hosts_hit")
		l.WithFields(log.Fields{
			"request_id": r.Id}).Infoln("Hit hosts file")
		return
	}

	// Found it in postive cache
	if r.Question[0].Qtype == dns.TypeA {
		if item, found := DNSCache.Get(fmt.Sprintf("%s|%d", r.Question[0].Name, r.Question[0].Qtype)); found {
			temp := item.Expiration - time.Now().Unix()

			// return found content if its ttl is reasonable
			if temp > 0 {
				StatsD.Increment("Pcache_hit")
				n := new(dns.Msg)
				n.Answer = item.Content
				// format legal head for reply
				if r.RecursionDesired {
					n.MsgHdr.RecursionAvailable = true
				}
				// Deduce ttl with time pass
				for i := range n.Answer {
					n.Answer[i].Header().Ttl = uint32(temp)
					n.Answer[i].Header().Name = r.Question[0].Name
				}
				n.SetReply(r)
				l.WithFields(log.Fields{
					"request_id": r.Id}).Infof("Found A record in Cache for %v", n)
				w.WriteMsg(n)
				return
			}
		}

	}

	// Found it in negative cache
	if r.Question[0].Qtype == dns.TypeAAAA {
		if _, found := NegativeCache.Get(r.Question[0].Name); found {
			n := new(dns.Msg)
			n.SetReply(r)
			if r.RecursionDesired {
				n.MsgHdr.RecursionAvailable = true
			}
			l.WithFields(log.Fields{
				"request_id": r.Id}).Infof("Hit negative %s\n", r.Question[0].Name)
			StatsD.Increment("Ncache_hit")
			w.WriteMsg(n)
			return
		}
	}

	// Get the time right now
	getRequestTime := time.Now()

	// not buffered: send immediately
	StatsD.Increment("request_hit")
	EdnsChannel := make(chan *dns.Msg, len(CoreDNSForward.List()))
	HdnsChannel := make(chan *dns.Msg, 2)

	// max timeout
	mx := time.NewTimer(time.Duration(1) * time.Second)

	// If With http ON
	if c.WithHttpDNS == true {
		t := time.NewTimer(time.Duration(c.RequestTimeout) * time.Millisecond)

		// Handle type A request
		if r.Question[0].Qtype == dns.TypeA {
			// Maybe it's unnecessary
			httpDNSr := deepcopy.Copy(r).(*dns.Msg)
			eDNSr := deepcopy.Copy(r).(*dns.Msg)
			gDNSr := deepcopy.Copy(r).(*dns.Msg)

			// EDNS request goroutine
			go univForwarder(w, eDNSr, EdnsChannel)
			if c.HasGoogleDNSIP == true {
				go GoogleDNSForward.GForward(DNSCache, gDNSr, HdnsChannel)
			} else {
				go AliDNSForward.AliForward(DNSCache, httpDNSr, HdnsChannel)
			}

			for {
				select {
				case m := <-HdnsChannel:
					if r.RecursionDesired {
						m.MsgHdr.RecursionAvailable = true
					}
					m.SetReply(r)
					w.WriteMsg(m)
					l.WithFields(log.Fields{
						"request_id": r.Id}).Infoln("Hitt None Cached HTTP DNS Handler Before Time Out")
					l.WithFields(log.Fields{
						"request_id": r.Id}).Infoln(m)
					StatsD.Increment("http_hit")
					StatsD.Increment("has_respondes")
					StatsD.Time("http_respond_time", time.Now().Sub(getRequestTime))
					return
				case <-t.C:
					select {
					case em := <-EdnsChannel:
						if utility.CheckBlockedIP(em) == false {
							if r.RecursionDesired {
								em.MsgHdr.RecursionAvailable = true
							}
							w.WriteMsg(em)
							StatsD.Increment("edns_hit")
							StatsD.Increment("has_respondes")
							StatsD.Time("edns_respond_time", time.Now().Sub(getRequestTime))
							l.WithFields(log.Fields{
								"request_id": r.Id}).Infoln("Hitt None Cached EDNS Handler After Time Out")
							l.WithFields(log.Fields{
								"request_id": r.Id}).Infoln(em)
							return
						}
					case <-mx.C:
						dns.HandleFailed(w, r)
						l.WithFields(log.Fields{
							"request_id": r.Id}).Warnf("Timeout! for %s and Type %d", r.Question[0].Name, r.Question[0].Qtype)
						return
					}
				case <-mx.C:
					dns.HandleFailed(w, r)
					if r.Question[0].Qtype == dns.TypeAAAA {
						NegativeCache.Add(r.Question[0].Name, 0, 0)
					}
					l.WithFields(log.Fields{
						"request_id": r.Id}).Warnf("Timeout! for %s and Type %d", r.Question[0].Name, r.Question[0].Qtype)
					return
				}
			}
		}

		// Handle the rest of request otherwise
		go univForwarder(w, r, EdnsChannel)
		for {
			select {
			case em := <-EdnsChannel:
				if utility.CheckBlockedIP(em) == false {
					if r.RecursionDesired {
						em.MsgHdr.RecursionAvailable = true
					}
					w.WriteMsg(em)
					StatsD.Increment("edns_hit")
					StatsD.Increment("has_respondes")
					StatsD.Time("edns_respond_time", time.Now().Sub(getRequestTime))
					l.WithFields(log.Fields{
						"request_id": r.Id}).Infoln("Hitt None Cached EDNS Handler After Time Out")
					l.WithFields(log.Fields{
						"request_id": r.Id}).Infoln(em)
					return
				}
			case <-mx.C:
				dns.HandleFailed(w, r)
				if r.Question[0].Qtype == dns.TypeAAAA {
					NegativeCache.Add(r.Question[0].Name, 0, 0)
				}
				l.Warnf("Timeout! for %s and Type %d", r.Question[0].Name, r.Question[0].Qtype)
				return
			}
		}
	}
	go univForwarder(w, r, EdnsChannel)
	for {
		select {
		case em := <-EdnsChannel:
			if utility.CheckBlockedIP(em) == false {
				if r.RecursionDesired {
					em.MsgHdr.RecursionAvailable = true
				}
				l.WithFields(log.Fields{
					"request_id": r.Id}).Infoln("Hitt EDNS Handler With A None A Request")
				w.WriteMsg(em)
				StatsD.Increment("edns_hit")
				StatsD.Increment("has_respondes")
				StatsD.Time("edns_respond_time", time.Now().Sub(getRequestTime))
				return
			}
			l.WithFields(log.Fields{
				"request_id": r.Id}).Errorln("Channel somehow closed")
			return
		case <-mx.C:
			dns.HandleFailed(w, r)
			l.WithFields(log.Fields{
				"request_id": r.Id}).Warnf("Timeout! for %s and Type %d", r.Question[0].Name, r.Question[0].Qtype)
			if r.Question[0].Qtype == dns.TypeAAAA {
				NegativeCache.Add(r.Question[0].Name, 0, 0)
			}
			return
		}
	}
}
