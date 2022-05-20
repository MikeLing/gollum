# Gollum

* Layout
    + cache
    + doh
    + forward
    + hosts
    + utility

> cache: cache for the DNS content storage. It's a custorm LRU cache and will refresh every cached DNS content when the TTL comes to the end. 
> <br>
> doh: module of http/https dns handler for Alibaba/Google http/https dns resolver based on the config.
> <br>
> forwards: handler for EDNS resolver if necessary.
> <br>
> hosts: module for /etc/hosts file loading.
> utility: some tools and utilities blocks for gollum, just pay attention to the netif.go where is the virtual interface happened.
----

## how to use it:
The gollum needs a config file before you run it as your DNS resolver. e.g:
>{
> <br>
>    "bindAddr": "169.254.1.153", // the IP address where the gollum listening && service
> <br>
>    "bindPort": "53",  // listening port
> <br>
>    "nsnames": ["8.8.8.8:53", "114.114.114.114:53"], // EDNS resolvers when the failback happends.
> <br>
>    "hTTPDnServer": "http://203.107.1.33/[UID]/sign_d?host=%s&t=%d&s=%x", // Alibaba http dns request endpoint.
> <br>
>     "aliSecretKey": "XXXXXX", // alibaba secretkey for request
> <br>
>    "logpath": "/dev/log", // as it calls
> <br>
>    "requestTimeout": 100,  // waiting time before failback happends
> <br>
>    "cacheDefaultTimeout": 300, // if the cache haven't been hit after cacheDefaultTimeout, it will be cleaned up.
> <br>
>    "cleanInterval":1,  // the janitor process running interval
> <br>
>    "reportInterval":20, // config for netdate report Interval
> <br>
>    "withHttpDNS": true, // will the http dns will be enable(seriously?)
> <br>
>    "googleDnServer": false, 
> <br>
>    "googleDnSUrl": "https://dns.google.com/resolve",
> <br>
>    "region": "cn" // region indicator.
> <br>
> }


Note:

The region is necessary and important. The gollum will decide which http/https DNS upstream will be used in the future.
> na: googleHttpDns
> <br>
> cn: alihttpDns