package utility

import (
	"crypto/md5"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

// GenerateSign generate signature for ali httpdns
func GenerateSign(host, HTTPDnServer string) string {
	c := GetConfig()
	t := time.Now().UTC().Add(time.Second * time.Duration(86200)).Unix()
	// remove the last "." from host
	secretkey := c.AliSecretKey
	singStr := fmt.Sprintf("%s-%s-%d", host, secretkey, t)
	url := fmt.Sprintf(HTTPDnServer, host, t, md5.Sum([]byte(singStr)))
	return url
}

// CheckBlockedIP will check if ip in blacklist
func CheckBlockedIP(r *dns.Msg) bool {
	c := GetConfig()
	l := GetLogger()
	if len(r.Answer) > 0 {
		for _, a := range r.Answer {
			t := strings.Split(a.String(), "\t")
			answerIP := t[len(t)-1]
			for _, i := range c.BlackList {
				if i == answerIP {
					l.Warnf("Found Blocked IP %s \n", answerIP)
					return true
				}
			}
		}
	}

	return false
}

// IsIpv4 check if it's an ipv4 address
func IsIpv4(host string) bool {
	parts := strings.Split(host, ".")
	if len(parts) < 4 {
		return false
	}

	for _, x := range parts {
		if i, err := strconv.Atoi(x); err == nil {
			if i < 0 || i > 255 {
				return false
			}
		} else {
			return false
		}
	}
	return true
}
