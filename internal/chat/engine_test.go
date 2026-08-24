package chat

import (
	"encoding/json"
	"net"
	"testing"
)

func TestProtocolDialectsAcceptSupportedTuples(t *testing.T) {
	for _, dialect := range protocolDialects {
		message := wireMessage{Protocol: dialect.Name, Major: dialect.Major, MinMajor: dialect.Major, Magic: dialect.Magic}
		got, ok := protocolDialectForMessage(message)
		if !ok || got != dialect {
			t.Fatalf("dialect %+v was not accepted: %+v, %v", dialect, got, ok)
		}
	}
	for _, message := range []wireMessage{
		{Protocol: "unknown", Major: 2, Magic: DiscoveryMagic},
		{Protocol: "dzhgo", Major: 2, Magic: "WRONG_MAGIC"},
		{Protocol: "POPChat", Major: 2, Magic: "POPCHAT_DISCOVERY_V1"},
		{Protocol: "POPChat", Major: 1, MinMajor: 2, Magic: "POPCHAT_DISCOVERY_V1"},
	} {
		if _, ok := protocolDialectForMessage(message); ok {
			t.Fatalf("unsupported dialect tuple was accepted: %+v", message)
		}
	}
}

func TestProtocolDialectsForPeerReuseStoredDialect(t *testing.T) {
	got := protocolDialectsForPeer(Peer{ProtocolName: "POPChat", ProtocolMajor: 1, DiscoveryMagic: "POPCHAT_DISCOVERY_V1"})
	if len(got) != 1 || got[0].Name != "POPChat" {
		t.Fatalf("stored peer dialect was not reused: %+v", got)
	}
	if got := protocolDialectsForPeer(Peer{}); len(got) != len(protocolDialects) {
		t.Fatalf("unknown peer should use all fallback dialects: %+v", got)
	}
}

func TestHelloMessageForDialectUsesCompatibleCapabilities(t *testing.T) {
	engine := NewEngine()
	for _, dialect := range protocolDialects {
		message := engine.helloMessageForDialect("hello", dialect)
		if message.Protocol != dialect.Name || message.Major != dialect.Major || message.Magic != dialect.Magic {
			t.Fatalf("hello did not use dialect %+v: %+v", dialect, message)
		}
		if !hasCapability(message.Capabilities, "text") || !hasCapability(message.Capabilities, "image") || !hasCapability(message.Capabilities, "file") {
			t.Fatalf("common capabilities missing for %+v: %v", dialect, message.Capabilities)
		}
		if dialect.Major == 1 && hasCapability(message.Capabilities, "file-progress-v1") {
			t.Fatalf("POPChat/v1 should not advertise v2 capabilities: %v", message.Capabilities)
		}
	}
}

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

	if err := writeWire(client, wireMessage{Protocol: ProtocolName, Major: ProtocolMajor, MinMajor: ProtocolMajor, Magic: DiscoveryMagic, Type: "discover", DeviceID: "remote-device"}); err != nil {
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

func TestUpdatePeerRelationRefreshesCachedPeer(t *testing.T) {
	engine := NewEngine()
	engine.peers["peer-1"] = Peer{DeviceID: "peer-1", Relation: DiscoveredState}
	engine.updatePeerRelation("peer-1", PeerRelation)
	if engine.peers["peer-1"].Relation != PeerRelation {
		t.Fatalf("cached relation = %q, want %q", engine.peers["peer-1"].Relation, PeerRelation)
	}
}

func TestHandleOfflineKeepsFriendAndMarksItOffline(t *testing.T) {
	engine := NewEngine()
	engine.peers["friend-1"] = Peer{DeviceID: "friend-1", Relation: PeerRelation, Online: true}

	engine.handleOffline("friend-1")

	peer, ok := engine.peers["friend-1"]
	if !ok {
		t.Fatal("friend must remain in the peer cache")
	}
	if peer.Relation != PeerRelation {
		t.Fatalf("relation = %q, want %q", peer.Relation, PeerRelation)
	}
	if peer.Online {
		t.Fatal("friend must be marked offline")
	}
}

func TestHandleOfflineRemovesDiscoveredPeer(t *testing.T) {
	engine := NewEngine()
	engine.peers["discovered-1"] = Peer{DeviceID: "discovered-1", Relation: DiscoveredState, Online: true}

	engine.handleOffline("discovered-1")

	if _, ok := engine.peers["discovered-1"]; ok {
		t.Fatal("non-friend must be removed after an offline event")
	}
}

func containsIP(values []net.IP, want string) bool {
	for _, value := range values {
		if value.String() == want {
			return true
		}
	}
	return false
}
