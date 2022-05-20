package cache

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	lru "github.com/hashicorp/golang-lru"
	"github.com/miekg/dns"
)

// DNSRecord is content-Type of DNS record
type DNSRecord struct {
	Content          []dns.RR
	OrignalTimestamp int64
	Expiration       int64
}

// DefaultExpiration is the default expiration time
const DefaultExpiration time.Duration = 5 * time.Minute

// DNSRecorder will take care of dns record and responding
type DNSRecorder struct {
	DefaultExpiration time.Duration
	DNSItems          *lru.TwoQueueCache
	mu                sync.RWMutex
}

// Set will set an item to the cache, replacing any existing item.
func (d *DNSRecorder) Set(r []dns.RR, t time.Duration, qtype uint16) {
	// Expiration time of item
	var e int64

	if t > 0 {
		e = time.Now().Add(t).Unix()
	} else {
		e = time.Now().Add(d.DefaultExpiration).Unix()
	}

	item := DNSRecord{
		Content:          r,
		OrignalTimestamp: time.Now().Unix(),
		Expiration:       e,
	}

	// Use domain+tpye more like a unique
	// hash which can faster the querying process
	key := fmt.Sprintf("%s|%d", strings.ToLower(r[0].Header().Name), qtype)
	d.DNSItems.Add(key, item)
}

// Flush will clean the DNS recorders up
func (d *DNSRecorder) Flush() {
	d.DNSItems.Purge()
}

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (d *DNSRecorder) Get(k string) (DNSRecord, bool) {
	// "Inlining" of get and Expired
	defer sentry.Recover()
	i := strings.ToLower(k)
	item, found := d.DNSItems.Get(i)
	record, _ := item.(DNSRecord)

	if !found {
		return DNSRecord{}, false
	}

	if record.Expiration > 0 {
		if time.Now().Unix() > record.Expiration {
			return DNSRecord{}, false
		}
	}
	return record, true
}

// Delete an item from the cache. Does nothing if the key is not in the cache.
func (d *DNSRecorder) Delete(k string) {
	if d.DNSItems.Contains(k) {
		d.DNSItems.Remove(k)
	}
}

// ItemCount will returns the number of items in the cache. This may include items that have
// expired, but have not yet been cleaned up.
func (d *DNSRecorder) ItemCount() int {
	n := d.DNSItems.Len()
	return n
}

// Keys returns a slice of the keys in the cache.
// The frequently used keys are first in the returned slice.
func (d *DNSRecorder) Keys() []string {
	var ks []string
	for _, i := range d.DNSItems.Keys() {
		ks = append(ks, i.(string))
	}
	return ks
}

func newCache(de time.Duration, m *lru.TwoQueueCache) *DNSRecorder {
	if de == 0 {
		// the default expiration is 5 minutes
		de = 5 * time.Minute
	}
	c := &DNSRecorder{
		DefaultExpiration: de,
		DNSItems:          m,
	}
	return c
}

func newCacheWithJanitor(de time.Duration, ci time.Duration, m *lru.TwoQueueCache) *DNSRecorder {
	cache := newCache(de, m)
	// if ci > 0 {
	// 	runJanitor(cache, ci)
	// 	runtime.SetFinalizer(cache, stopJanitor)
	// }
	return cache
}

// NewDNSCache returns a new cache with a given default expiration duration and cleanup
// interval.
func NewDNSCache(defaultExpiration, cleanupInterval time.Duration) *DNSRecorder {
	items, _ := lru.New2Q(100)
	return newCacheWithJanitor(defaultExpiration, cleanupInterval, items)
}
