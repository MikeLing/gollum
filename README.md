[![Build Status](https://travis-ci.com/MikeLing/gollum.svg?token=RssszaccxtoAKJp35dcy&branch=master)](https://travis-ci.com/MikeLing/gollum) 
[![codecov](https://codecov.io/gh/MikeLing/gollum/branch/master/graph/badge.svg?token=zpGpDL49Kg)](https://codecov.io/gh/MikeLing/gollum)

# Gollum
A DNS Resolver which combined DNS(EDNS) and HTTP/HTTPS(ali http dns upstream only for now) DNS.

## What's the HTTP DNS and Why we need it?

### The DNS querying process could be exploited and vulnerable
Usually a resolver will tell each DNS server what domain you are looking for. This request sometimes includes your full IP address. Or if not your full IP address, increasingly often the request includes most of your IP address, which can easily be combined with other information to figure out your identity.

This means that every server that you ask to help with domain name resolution sees what site you’re looking for. But more than that, it also means that anyone on the path to those servers sees your requests, too.I t’s easy to take the full or partial IP address info and figure out who’s asking for that web site.

And that data is valuable. Many people and companies will pay lots of money to see what you are browsing for.

![a router offering to sell data](https://hacks.mozilla.org/files/2018/05/03_02-500x295.png)

With spoofing, someone on the path between the DNS server and you changes the response. Instead of telling you the real IP address, a spoofer will give you the wrong IP address for a site. This way, they can block you from visiting the real site or send you to a scam one.

![spoofer sending user to wrong site](https://hacks.mozilla.org/files/2018/05/03_03-500x295.png)

### DNS over HTTP/HTTPS
DNS over HTTPS (DoH) is a protocol for performing remote Domain Name System (DNS) resolution via the HTTPS protocol. A goal of the method is to increase user privacy and security by preventing eavesdropping and manipulation of DNS data by man-in-the-middle attacks. As of March 2018, Google and the Mozilla Foundation are testing versions of DNS over HTTPS. But HTTP/HTTPS DNS currently lacks native support in operating systems. Thus a user wishing to use it must install additional software which is what's the **Gollum** for!

> Read more about HTTP/HTTPS DNS in [here](https://en.wikipedia.org/wiki/DNS_over_HTTPS) and [here](https://hacks.mozilla.org/2018/05/a-cartoon-intro-to-dns-over-https/)

## How Gollum did it?
Basically speaking, Gollum is a DNS solver based on https://github.com/miekg/dns and [coredns](https://github.com/coredns/coredns).

![logic of gollum](/projects/IPT/repos/gollum/browse/pictures/1.jpg)

The gollum will send EDNS request and HTTP DNS request to upstream at the same time when a DNS request arrive. But Gollum conside the HTTP upstream answer in the first priority and wait for `100ms` if the HTTP DNS upstream haven't give answer in time. After that, the Gollum will return response with any answer (except ip in blacklist). But only HTTP DNS upstream answer will be used to update cache.

![gollum flow](/projects/IPT/repos/gollum/browse/pictures/2.png)
