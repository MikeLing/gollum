// netif will create a virtual interface for ip addresses bounding and request overflow.
package utility

import (
	"fmt"

	"github.com/vishvananda/netlink"
)

// AddDummyDevice creates a dummy device with the given name. It also binds the ip address of the NetifManager instance
// to this device. This function returns an error if the device exists or if address binding fails.
func addDummyDevice(ip string) error {
	la := netlink.NewLinkAttrs()
	la.Name = "gollum"
	// Create dummy interface
	mybridge := &netlink.Bridge{LinkAttrs: la}
	err := netlink.LinkAdd(mybridge)
	if err != nil {
		fmt.Printf("could not add %s: %v\n", la.Name, err)
	}

	// Bound address on it
	nl, err := netlink.LinkByName(la.Name)
	netlink.LinkSetMaster(nl, mybridge)
	addr, _ := netlink.ParseAddr(ip)
	netlink.AddrAdd(nl, addr)

	return err
}

// EnsureDummyDevice checks for the presence of the given dummy device and creates one if it does not exist.
// Returns a boolean to indicate if this device was found and error if any.
func EnsureDummyDevice(ip string) (bool, error) {
	l, err := netlink.LinkByName("gollum")
	if err == nil {
		// found dummy device, make sure ip matches. AddrAdd will return error if address exists, will add it otherwise
		addr, _ := netlink.ParseAddr(ip)
		netlink.AddrAdd(l, addr)
		return true, nil
	}
	return false, addDummyDevice(ip)
}

// RemoveDummyDevice deletes the dummy device with the given name.
func RemoveDummyDevice() error {
	link, err := netlink.LinkByName("gollum")
	if err != nil {
		return err
	}
	return netlink.LinkDel(link)
}
