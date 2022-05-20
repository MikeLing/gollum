// janitor is a logic block that for renew outdated dns records and the check interval is 1s
// the stander of dns outdateing is based on TTL of DNS records.

package main

import (
	"strconv"
	"strings"
	"time"

	dcache "github.com/gollum/gollum/cache"
	"github.com/gollum/gollum/utility"
	"github.com/miekg/dns"
)

type janitor struct {
	Interval time.Duration
	stop     chan bool
}

func (j *janitor) reNewExpired(key string, d *dcache.DNSRecorder) bool {
	// max timeout
	m := time.NewTimer(time.Duration(1) * time.Second)
	c := utility.GetConfig()
	domainT := strings.Split(key, "|")
	domain := domainT[0]
	qtype, _ := strconv.Atoi(domainT[1])
	if c.HasGoogleDNSIP == true {
		temp := make(chan *dns.Msg, 1)
		msg := new(dns.Msg)
		msg.SetQuestion(domain, uint16(qtype))
		go GoogleDNSForward.GForward(d, msg, temp)
		// Do nothing in here, just for update cache
		select {
		case <-temp:
			return true
		case <-m.C:
			return false
		}
	} else {
		temp := make(chan *dns.Msg, 1)
		msg := new(dns.Msg)
		msg.SetQuestion(domain, uint16(qtype))
		go AliDNSForward.AliForward(d, msg, temp)
		// Do nothing in here, just for update cache
		select {
		case <-temp:
			return true
		case <-m.C:
			return false
		}
	}
}

// ReNewExpired will renew all expired items from the cache.
func (j *janitor) ReNewExpired(d *dcache.DNSRecorder) {
	now := time.Now().Unix()
	l := utility.GetLogger()
	if d.ItemCount() > 0 {
		for _, v := range d.DNSItems.Keys() {
			itemI, _ := d.DNSItems.Peek(v)
			item, _ := itemI.(dcache.DNSRecord)
			if item.Expiration > 0 && now > item.Expiration {
				sucess := j.reNewExpired(v.(string), d)
				if sucess == false {
					l.Errorln("Failed to Renew")
				}
			}
		}
	}
}

func (j *janitor) Run(d *dcache.DNSRecorder) {
	ticker := time.NewTicker(j.Interval)
	for {
		select {
		case <-ticker.C:
			j.ReNewExpired(d)
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

func (j *janitor) StopJanitor() {
	j.stop <- true
}
