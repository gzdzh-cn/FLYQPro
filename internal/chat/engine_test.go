package chat

import (
	"encoding/json"
	"net"
	"testing"
)

func TestSubnetHostTargetsIncludesPeerAndExcludesLocalAndBroadcast(t *testing.T) {
	_, subnet, err := net.ParseCIDR("192.168.43.4/24")
	if err != nil {
		t.Fatal(err)
	}
	subnet.IP = net.ParseIP("192.168.43.4")

	targets := subnetHostTargets(subnet)
	if len(targets) != 253 {
		t.Fatalf("target count = %d, want 253", len(targets))
	}
	if containsIP(targets, "192.168.43.4") {
		t.Fatal("local address must not be probed")
	}
	if containsIP(targets, "192.168.43.0") || containsIP(targets, "192.168.43.255") {
		t.Fatal("network and broadcast addresses must not be probed")
	}
	if !containsIP(targets, "192.168.43.5") {
		t.Fatal("reachable peer address must be probed")
	}
}

func TestSubnetHostTargetsSkipsLargeNetworks(t *testing.T) {
	_, subnet, err := net.ParseCIDR("10.0.0.1/16")
	if err != nil {
		t.Fatal(err)
	}
	subnet.IP = net.ParseIP("10.0.0.1")
	if targets := subnetHostTargets(subnet); len(targets) != 0 {
		t.Fatalf("large subnet produced %d targets", len(targets))
	}
}

func TestHandleDiscoveryTCPRepliesWhenDiscoverable(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	engine := &Engine{
		profile:  Profile{Discoverable: true, Nickname: "测试设备"},
		identity: Identity{DeviceInfo: DeviceInfo{DeviceID: "local-device"}},
	}
	done := make(chan struct{})
	go func() {
		engine.handleDiscoveryTCP(server)
		close(done)
	}()

	if err := writeWire(client, wireMessage{Magic: DiscoveryMagic, Type: "discover", DeviceID: "remote-device"}); err != nil {
		t.Fatal(err)
	}
	var response wireMessage
	if err := json.NewDecoder(client).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Magic != DiscoveryMagic || response.Type != "announce" || response.Nickname != "测试设备" {
		t.Fatalf("unexpected discovery response: %+v", response)
	}
	<-done
}

func containsIP(values []net.IP, want string) bool {
	for _, value := range values {
		if value.String() == want {
			return true
		}
	}
	return false
}
