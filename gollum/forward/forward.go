// Package forward implements a forwarding proxy. It caches an upstream net.Conn for some time, so if the same
// client returns the upstream's Conn will be precached. Depending on how you benchmark this looks to be
// 50% faster than just opening a new connection for every client. It works with UDP and TCP and uses
// inband healthchecking.
package forward

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/debug"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/getsentry/sentry-go"

	"github.com/miekg/dns"
	ot "github.com/opentracing/opentracing-go"
)

var log = clog.NewWithPlugin("forward")

// Forward represents a plugin instance that can proxy requests to another (DNS) server. It has a list
// of proxies each representing one upstream proxy.
type Forward struct {
	proxies    []*Proxy
	p          Policy
	hcInterval time.Duration

	from    string
	ignored []string

	tlsConfig     *tls.Config
	tlsServerName string
	maxfails      uint32
	expire        time.Duration

	opts Options // also here for testing

	Next plugin.Handler
}

// New returns a new Forward.
func New() *Forward {
	f := &Forward{maxfails: 2, tlsConfig: new(tls.Config), expire: defaultExpire, p: new(random), from: ".", hcInterval: hcInterval}
	return f
}

// SetProxy appends p to the proxy list and starts healthchecking.
func (f *Forward) SetProxy(p *Proxy) {
	f.proxies = append(f.proxies, p)
	p.start(f.hcInterval)
}

// Len returns the number of configured proxies.
func (f *Forward) Len() int { return len(f.proxies) }

// Name implements plugin.Handler.
func (f *Forward) Name() string { return "forward" }

// ServeDNS implements plugin.Handler.
func (f *Forward) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg, EdnsChannel chan *dns.Msg) {
	defer sentry.Recover()
	state := request.Request{W: w, Req: r}

	var span, child ot.Span
	span = ot.SpanFromContext(ctx)
	list := f.List()

	wg := sync.WaitGroup{}
	wg.Add(len(list))
	for _, p := range list {
		go func(proxy *Proxy) {

			// If proxy broken more than maxfaills, just ignore it
			if proxy.Down(f.maxfails) {
				wg.Done()
				return
			}

			if span != nil {
				child = span.Tracer().StartSpan("connect", ot.ChildOf(span.Context()))
				ctx = ot.ContextWithSpan(ctx, child)
			}

			var (
				ret *dns.Msg
				err error
			)
			opts := f.opts
			for {
				ret, err = proxy.Connect(ctx, state)
				if err == nil {
					break
				}
				if err == ErrCachedClosed { // Remote side closed conn, can only happen with TCP.
					continue
				}
				// Retry with TCP if truncated and prefer_udp configured.
				if ret != nil && ret.Truncated && !opts.forceTCP && f.opts.preferUDP {
					opts.forceTCP = true
					continue
				}
				break
			}

			if child != nil {
				child.Finish()
			}
			//taperr := toDnstap(ctx, proxy.addr, f, state, ret, start)

			if err != nil {
				// Kick off health check to see if *our* upstream is broken.
				if f.maxfails != 0 {
					proxy.Healthcheck()
				}
				wg.Done()
				return
			}
			// Check if the reply is correct
			if !state.Match(ret) {
				debug.Hexdumpf(ret, "Wrong reply for id: %d, %s %d", ret.Id, state.QName(), state.QType())

				formerr := new(dns.Msg)
				formerr.SetRcode(state.Req, dns.RcodeFormatError)
				// w.WriteMsg(formerr)
				log.Error(formerr)
				wg.Done()
				return
			}

			// w.WriteMsg(ret)
			select {
			case EdnsChannel <- ret:
			default:
			}
			wg.Done()
			return
		}(p)
	}
	wg.Wait()

}

func (f *Forward) match(state request.Request) bool {
	if !plugin.Name(f.from).Matches(state.Name()) || !f.isAllowedDomain(state.Name()) {
		return false
	}

	return true
}

func (f *Forward) isAllowedDomain(name string) bool {
	if dns.Name(name) == dns.Name(f.from) {
		return true
	}

	for _, ignore := range f.ignored {
		if plugin.Name(ignore).Matches(name) {
			return false
		}
	}
	return true
}

// ForceTCP returns if TCP is forced to be used even when the request comes in over UDP.
func (f *Forward) ForceTCP() bool { return f.opts.forceTCP }

// PreferUDP returns if UDP is preferred to be used even when the request comes in over TCP.
func (f *Forward) PreferUDP() bool { return f.opts.preferUDP }

// List returns a set of proxies to be used for this client depending on the policy in f.
func (f *Forward) List() []*Proxy { return f.p.List(f.proxies) }

var (
	// ErrNoHealthy means no healthy proxies left.
	ErrNoHealthy = errors.New("no healthy proxies")
	// ErrNoForward means no forwarder defined.
	ErrNoForward = errors.New("no forwarder defined")
	// ErrCachedClosed means cached connection was closed by peer.
	ErrCachedClosed = errors.New("cached connection was closed by peer")
)

// policy tells forward what policy for selecting upstream it uses.
type policy int

const (
	randomPolicy policy = iota
	roundRobinPolicy
	sequentialPolicy
)

// options holds various options that can be set.
type Options struct {
	forceTCP  bool
	preferUDP bool
}

const defaultTimeout = 5 * time.Second
