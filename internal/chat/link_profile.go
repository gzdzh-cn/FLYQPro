package chat

import (
	"net"
	"sort"
	"strings"
)

type LinkProfileV3 struct {
	Type                             LinkType
	SpeedMbps                        int
	RTTMillis                        int64
	LossRate                         float64
	InterfaceName, LocalIP, RemoteIP string
}

// DetectLocalLinkProfile uses the portable interface API. Link speed is left
// at zero because net.Interface does not expose it consistently across macOS
// and Windows; a platform-specific probe can override it later.
func DetectLocalLinkProfile(localIP string) (LinkProfileV3, []string) {
	profile := LinkProfileV3{Type: LinkUnknown, LocalIP: localIP}
	addresses := make([]string, 0)
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		ifaceAddresses := interfaceIPs(iface)
		if len(ifaceAddresses) == 0 {
			continue
		}
		addresses = append(addresses, ifaceAddresses...)
		matches := localIP == ""
		for _, ip := range ifaceAddresses {
			matches = matches || ip == localIP
		}
		if matches && profile.InterfaceName == "" {
			profile.InterfaceName = iface.Name
			profile.Type = classifyInterfaceName(iface.Name)
		}
	}
	sort.Strings(addresses)
	return profile, addresses
}

func interfaceIPs(iface net.Interface) []string {
	list, err := iface.Addrs()
	if err != nil {
		return nil
	}
	result := make([]string, 0, len(list))
	for _, addr := range list {
		if ip, _, err := net.ParseCIDR(addr.String()); err == nil {
			result = append(result, ip.String())
		}
	}
	return result
}

func classifyInterfaceName(name string) LinkType {
	name = strings.ToLower(name)
	for _, marker := range []string{"wi-fi", "wifi", "wlan", "airport", "en0", "en1"} {
		if strings.Contains(name, marker) {
			return LinkWiFi
		}
	}
	for _, marker := range []string{"ethernet", "eth", "enp", "eno", "ens", "bridge"} {
		if strings.Contains(name, marker) {
			return LinkEthernet
		}
	}
	return LinkUnknown
}

func (p LinkProfileV3) SlotLimit(size int64) int {
	if size < 8<<20 {
		return 1
	}
	if p.Type == LinkEthernet && p.SpeedMbps >= 1000 {
		return 4
	}
	if p.Type == LinkEthernet {
		return 2
	}
	// Discovery may not yet have a reliable interface speed. Scale by file
	// size in that case so large transfers still fill a gigabit-class link,
	// while small files avoid connection setup overhead.
	if size >= 1<<30 {
		return 4
	}
	if size >= 64<<20 {
		return 4
	}
	return 2
}
func (p LinkProfileV3) ChunkBytes(size int64) int {
	if size < 8<<20 {
		return 256 << 10
	}
	if p.Type == LinkEthernet && p.SpeedMbps >= 1000 {
		return 1 << 20
	}
	if p.Type == LinkEthernet {
		return 256 << 10
	}
	return 512 << 10
}
