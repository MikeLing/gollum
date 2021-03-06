package forward

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/pkg/transport"
	"github.com/coredns/coredns/plugin/test"

	"github.com/miekg/dns"
)

func TestHealth(t *testing.T) {
	const expected = 0
	i := uint32(0)
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		if r.Question[0].Name == "." {
			atomic.AddUint32(&i, 1)
		}
		ret := new(dns.Msg)
		ret.SetReply(r)
		w.WriteMsg(ret)
	})
	defer s.Close()

	p := NewProxy(s.Addr, transport.DNS)
	f := New()
	f.SetProxy(p)
	defer f.Close()

	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeA)
	e := make(chan *dns.Msg, 1)

	f.ServeDNS(context.TODO(), &test.ResponseWriter{}, req, e)

	time.Sleep(1 * time.Second)
	i1 := atomic.LoadUint32(&i)
	if i1 != expected {
		t.Errorf("Expected number of health checks to be %d, got %d", expected, i1)
	}
}

func TestHealthTimeout(t *testing.T) {
	const expected = 1
	i := uint32(0)
	q := uint32(0)
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		if r.Question[0].Name == "." {
			// health check, answer
			atomic.AddUint32(&i, 1)
			ret := new(dns.Msg)
			ret.SetReply(r)
			w.WriteMsg(ret)
			return
		}
		if atomic.LoadUint32(&q) == 0 { //drop only first query
			atomic.AddUint32(&q, 1)
			return
		}
		ret := new(dns.Msg)
		ret.SetReply(r)
		w.WriteMsg(ret)
	})
	defer s.Close()

	p := NewProxy(s.Addr, transport.DNS)
	f := New()
	f.SetProxy(p)
	defer f.Close()

	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeA)
	e := make(chan *dns.Msg, 1)
	f.ServeDNS(context.TODO(), &test.ResponseWriter{}, req, e)

	time.Sleep(1 * time.Second)
	i1 := atomic.LoadUint32(&i)
	if i1 != expected {
		t.Errorf("Expected number of health checks to be %d, got %d", expected, i1)
	}
}

func TestHealthNoMaxFails(t *testing.T) {
	const expected = 0
	i := uint32(0)
	s := dnstest.NewServer(func(w dns.ResponseWriter, r *dns.Msg) {
		if r.Question[0].Name == "." {
			// health check, answer
			atomic.AddUint32(&i, 1)
			ret := new(dns.Msg)
			ret.SetReply(r)
			w.WriteMsg(ret)
		}
	})
	defer s.Close()

	p := NewProxy(s.Addr, transport.DNS)
	f := New()
	f.maxfails = 0
	f.SetProxy(p)
	defer f.Close()

	req := new(dns.Msg)
	req.SetQuestion("example.org.", dns.TypeA)

	e := make(chan *dns.Msg, 1)
	f.ServeDNS(context.TODO(), &test.ResponseWriter{}, req, e)

	time.Sleep(1 * time.Second)
	i1 := atomic.LoadUint32(&i)
	if i1 != expected {
		t.Errorf("Expected number of health checks to be %d, got %d", expected, i1)
	}
}
