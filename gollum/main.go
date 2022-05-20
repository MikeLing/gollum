package main

import (
	"fmt"
	_ "net/http/pprof"
	"runtime"
	"time"

	"github.com/gollum/gollum/doh"
	. "github.com/gollum/gollum/doh"
	"github.com/gollum/gollum/hosts"
	"github.com/gollum/gollum/utility"
	"github.com/netdata/go-statsd"

	"github.com/coredns/coredns/plugin/pkg/transport"
	dcache "github.com/gollum/gollum/cache"
	"github.com/gollum/gollum/forward"
	"github.com/miekg/dns"
	"github.com/patrickmn/go-cache"
)

var (
	// DNSCache is the cache for http dns records
	DNSCache *dcache.DNSRecorder

	// CoreDNSForward is a DNS Forward for unDNS requests
	CoreDNSForward *forward.Forward

	// AliDNSForward is a DNS Forward for unDNS requests
	AliDNSForward *AliDoH

	// GoogleDNSForward is a DNS Forward for unDNS requests
	GoogleDNSForward *GoogleDoH

	// GollumHosts is DNS Forward for hosts record
	GollumHosts *hosts.Hosts

	// NegativeCache is the negative dns cache
	NegativeCache *cache.Cache

	// StatsD client
	StatsD *statsd.Client
)

func init() {
	c := utility.GetConfig()
	// All the DNS cache will has 5 minutes default expired time
	DNSCache = dcache.NewDNSCache(time.Duration(c.CacheDefaultTimeout)*time.Second,
		time.Duration(c.CleanInterval)*time.Second)

	j := &janitor{
		Interval: time.Duration(1) * time.Second,
		stop:     make(chan bool),
	}

	go j.Run(DNSCache)

	NegativeCache = cache.New(20*time.Second, 5*time.Second)

	CoreDNSForward = forward.New()
	for _, addr := range c.Nsnames {
		p := forward.NewProxy(addr, transport.DNS)
		CoreDNSForward.SetProxy(p)
	}

	defer CoreDNSForward.Close()

	GollumHosts = hosts.NewHostsProxy()
	AliDNSForward = doh.GetADoH()
	GoogleDNSForward = doh.GetGDoH()

	statsWriter, err := statsd.UDP(":8125")
	if err != nil {
		panic(err)
	}
	StatsD = statsd.NewClient(statsWriter, "gollum_")
	StatsD.FlushEvery(1 * time.Second)
}

func main() {
	runtime.GOMAXPROCS(2)
	l := utility.GetLogger()
	c := utility.GetConfig()
	fmt.Println(c.HasGoogleDNSIP)
	fmt.Println(c.BindAddr)

	dummyInterfaceIP := fmt.Sprintf("%s/%s", c.BindAddr, "24")
	fmt.Println(dummyInterfaceIP)
	utility.EnsureDummyDevice(dummyInterfaceIP)

	// remoe dummy interface after exit
	defer utility.RemoveDummyDevice()
	l.Infoln("- - - - - - - - - - - - - - -")

	dns.HandleFunc(".", HandleRoot)
	err := dns.ListenAndServe(fmt.Sprintf("%s:%s", c.BindAddr, c.BindPort), "udp", nil)
	if err != nil {
		fmt.Println(err)
	}
}
