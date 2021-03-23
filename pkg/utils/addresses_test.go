package utils

import (
	"fmt"
	"net"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

var lo = &netlink.Device{
	LinkAttrs: netlink.LinkAttrs{
		Index: 0,
		Name:  "lo",
	},
}
var eth0 = &netlink.Device{
	LinkAttrs: netlink.LinkAttrs{
		Index: 1,
		Name:  "eth0",
	},
}
var eth1 = &netlink.Device{
	LinkAttrs: netlink.LinkAttrs{
		Index: 2,
		Name:  "eth1",
	},
}

func maybeAddAddress(addrMap map[netlink.Link][]netlink.Addr, af AddressFilter, link netlink.Link, addrStr string, deprecated bool) {
	addr, err := netlink.ParseAddr(addrStr)
	if err != nil {
		panic(fmt.Sprintf("bad address string %q in test case", addrStr))
	}
	if !deprecated {
		addr.PreferedLft = 999
	}
	if af != nil && !af(*addr) {
		return
	}
	addrMap[link] = append(addrMap[link], *addr)
}

func maybeAddRoute(routeMap map[int][]netlink.Route, rf RouteFilter, link netlink.Link, destination string, ra bool) {
	var dst *net.IPNet
	var err error
	if destination != "" {
		_, dst, err = net.ParseCIDR(destination)
		if err != nil {
			panic(fmt.Sprintf("bad route string %q in test case", destination))
		}
	}
	prot := unix.RTPROT_KERNEL
	if ra {
		prot = unix.RTPROT_RA
	}
	linkIndex := link.Attrs().Index
	route := netlink.Route{
		LinkIndex: linkIndex,
		Dst:       dst,
		Protocol:  prot,
	}
	if rf != nil && !rf(route) {
		return
	}
	routeMap[linkIndex] = append(routeMap[linkIndex], route)
}

func addIPv4Addrs(addrs map[netlink.Link][]netlink.Addr, af AddressFilter) {
	maybeAddAddress(addrs, af, lo, "127.0.0.1/8", false)
	maybeAddAddress(addrs, af, lo, "::1/128", false)
	maybeAddAddress(addrs, af, eth0, "10.0.0.5/24", false)
	maybeAddAddress(addrs, af, eth0, "169.254.10.10/16", false)
	maybeAddAddress(addrs, af, eth0, "10.0.0.100/24", false)
	maybeAddAddress(addrs, af, eth1, "192.168.1.2/24", false)
}

func addIPv4Routes(routes map[int][]netlink.Route, rf RouteFilter) {
	maybeAddRoute(routes, rf, eth0, "", false)
	maybeAddRoute(routes, rf, eth0, "10.0.0.0/24", false)
	maybeAddRoute(routes, rf, eth1, "192.168.1.0/24", false)
}

func addIPv4RoutesDefaultEth1(routes map[int][]netlink.Route, rf RouteFilter) {
	maybeAddRoute(routes, rf, eth0, "10.0.0.0/24", false)
	maybeAddRoute(routes, rf, eth1, "", false)
	maybeAddRoute(routes, rf, eth1, "192.168.1.0/24", false)
}

func addIPv6Addrs(addrs map[netlink.Link][]netlink.Addr, af AddressFilter) {
	maybeAddAddress(addrs, af, lo, "127.0.0.1/8", false)
	maybeAddAddress(addrs, af, lo, "::1/128", false)
	maybeAddAddress(addrs, af, eth0, "fd00::5/64", false)
	maybeAddAddress(addrs, af, eth0, "fe80::1234/64", false)
	maybeAddAddress(addrs, af, eth1, "fd01::3/64", true)
	maybeAddAddress(addrs, af, eth1, "fd01::4/64", true)
	maybeAddAddress(addrs, af, eth1, "fd01::5/64", false)
}

func addIPv6Routes(routes map[int][]netlink.Route, rf RouteFilter) {
	maybeAddRoute(routes, rf, eth0, "", false)
	maybeAddRoute(routes, rf, eth0, "fd00::/64", false)
	maybeAddRoute(routes, rf, eth0, "fd02::/64", false)
	maybeAddRoute(routes, rf, eth1, "fd01::/64", false)
}

func ipv4AddrMap(af AddressFilter) (map[netlink.Link][]netlink.Addr, error) {
	addrs := make(map[netlink.Link][]netlink.Addr)
	addIPv4Addrs(addrs, af)
	return addrs, nil
}

func ipv4RouteMap(rf RouteFilter) (map[int][]netlink.Route, error) {
	routes := make(map[int][]netlink.Route)
	addIPv4Routes(routes, rf)
	return routes, nil
}

func ipv4RouteMapDefaultEth1(rf RouteFilter) (map[int][]netlink.Route, error) {
	routes := make(map[int][]netlink.Route)
	addIPv4RoutesDefaultEth1(routes, rf)
	return routes, nil
}

func ipv6AddrMap(af AddressFilter) (map[netlink.Link][]netlink.Addr, error) {
	addrs := make(map[netlink.Link][]netlink.Addr)
	addIPv6Addrs(addrs, af)
	return addrs, nil
}

func ipv6RouteMap(rf RouteFilter) (map[int][]netlink.Route, error) {
	routes := make(map[int][]netlink.Route)
	addIPv6Routes(routes, rf)
	return routes, nil
}

func dualStackAddrMap(af AddressFilter) (map[netlink.Link][]netlink.Addr, error) {
	addrs := make(map[netlink.Link][]netlink.Addr)
	addIPv4Addrs(addrs, af)
	addIPv6Addrs(addrs, af)
	return addrs, nil
}

func dualStackRouteMap(rf RouteFilter) (map[int][]netlink.Route, error) {
	routes := make(map[int][]netlink.Route)
	addIPv4Routes(routes, rf)
	addIPv6Routes(routes, rf)
	return routes, nil
}

var _ = Describe("addresses", func() {
	It("matches an IPv4 VIP on the primary interface", func() {
		addrs, err := addressesRoutingInternal(
			[]net.IP{net.ParseIP("10.0.0.2")},
			ValidNodeAddress,
			ipv4AddrMap,
			ipv4RouteMap,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(addrs).To(Equal([]net.IP{net.ParseIP("10.0.0.5")}))
	})

	It("matches an IPv4 VIP on the secondary interface", func() {
		addrs, err := addressesRoutingInternal(
			[]net.IP{net.ParseIP("192.168.1.99")},
			ValidNodeAddress,
			ipv4AddrMap,
			ipv4RouteMap,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(addrs).To(Equal([]net.IP{net.ParseIP("192.168.1.2")}))
	})

	It("matches an IPv4 VIP when the default route is on another interface", func() {
		addrs, err := addressesRoutingInternal(
			[]net.IP{net.ParseIP("10.0.0.2")},
			ValidNodeAddress,
			ipv4AddrMap,
			ipv4RouteMapDefaultEth1,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(addrs).To(Equal([]net.IP{net.ParseIP("10.0.0.5")}))
	})

	It("matches an IPv6 VIP on the primary interface", func() {
		addrs, err := addressesRoutingInternal(
			[]net.IP{net.ParseIP("fd00::2")},
			ValidNodeAddress,
			ipv6AddrMap,
			ipv6RouteMap,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(addrs).To(Equal([]net.IP{net.ParseIP("fd00::5")}))
	})

	It("matches an IPv6 VIP on an interface with temporary IPs", func() {
		addrs, err := addressesRoutingInternal(
			[]net.IP{net.ParseIP("fd01::2")},
			ValidNodeAddress,
			ipv6AddrMap,
			ipv6RouteMap,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(addrs).To(Equal([]net.IP{net.ParseIP("fd01::5")}))
	})

	It("matches an IPv4 VIP on a dual-stack interface", func() {
		addrs, err := addressesRoutingInternal(
			[]net.IP{net.ParseIP("10.0.0.2")},
			ValidNodeAddress,
			dualStackAddrMap,
			dualStackRouteMap,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(addrs).To(Equal([]net.IP{net.ParseIP("10.0.0.5"), net.ParseIP("fd00::5")}))
	})

	It("matches an IPv6 VIP on a dual-stack interface", func() {
		addrs, err := addressesRoutingInternal(
			[]net.IP{net.ParseIP("fd01::2")},
			ValidNodeAddress,
			dualStackAddrMap,
			dualStackRouteMap,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(addrs).To(Equal([]net.IP{net.ParseIP("fd01::5"), net.ParseIP("192.168.1.2")}))
	})

	It("finds an interface with a default route in an IPv4 cluster", func() {
		addrs, err := addressesDefaultInternal(
			ValidNodeAddress,
			ipv4AddrMap,
			ipv4RouteMap,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(addrs).To(Equal([]net.IP{net.ParseIP("10.0.0.5")}))
	})

	It("finds an interface with a default route when that's not the first interface", func() {
		addrs, err := addressesDefaultInternal(
			ValidNodeAddress,
			ipv4AddrMap,
			ipv4RouteMapDefaultEth1,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(addrs).To(Equal([]net.IP{net.ParseIP("192.168.1.2")}))
	})

	It("finds an interface with a default route in an IPv6 cluster", func() {
		addrs, err := addressesDefaultInternal(
			ValidNodeAddress,
			ipv6AddrMap,
			ipv6RouteMap,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(addrs).To(Equal([]net.IP{net.ParseIP("fd00::5")}))
	})

	It("finds an interface with a default route in a dual-stack cluster", func() {
		addrs, err := addressesDefaultInternal(
			ValidNodeAddress,
			dualStackAddrMap,
			dualStackRouteMap,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(addrs).To(Equal([]net.IP{net.ParseIP("10.0.0.5"), net.ParseIP("fd00::5")}))
	})
})

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Addresses tests")
}
