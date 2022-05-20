package forward

import (
	"context"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/transport"
	"github.com/coredns/coredns/plugin/test"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

func TestProxyClose(t *testing.T) {
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		ret := new(dns.Msg)
		ret.SetReply(r)
		w.WriteMsg(ret)
	})
	defer s.Close()

	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeA)
	state := request.Request{W: &test.ResponseWriter{}, Req: req}
	ctx := context.TODO()

	for i := 0; i < 100; i++ {
		p := NewProxy(s.Addr, transport.DNS)
		p.start(hcInterval)

		go func() { p.Connect(ctx, state) }()
		go func() { p.Connect(ctx, state) }()
		go func() { p.Connect(ctx, state) }()
		go func() { p.Connect(ctx, state) }()

		p.close()
	}
}

func TestProxy(t *testing.T) {
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		ret := new(dns.Msg)
		ret.SetReply(r)
		ret.Answer = append(ret.Answer, test.A("example.org. IN A 127.0.0.1"))
		w.WriteMsg(ret)
	})
	defer s.Close()
	f := New()
	f.SetProxy(NewProxy(s.Addr, transport.DNS))
	maxTimer := time.NewTimer(time.Duration(1) * time.Second)
	m := new(dns.Msg)
	m.SetQuestion("example.org.", dns.TypeA)
	rec := dnstest.NewRecorder(&test.ResponseWriter{})

	e := make(chan *dns.Msg, 1)
	f.ServeDNS(context.TODO(), rec, m, e)
	select{
	case a := <- e:
		if x := a.Answer[0].Header().Name; x!="example.org." {
			t.Errorf("Expected %s, got %s", "example.org.", x)
		}
	case <- maxTimer.C:
		t.Fatal("Expected to receive reply, but didn't")
	}
}

