package chat

import (
	"encoding/json"
	"net"
	"testing"
)

func TestProtocolDialectAcceptsCanonicalTuple(t *testing.T) {
	message := wireMessage{Protocol: ProtocolName, Major: ProtocolMajor, MinMajor: ProtocolMajor, Magic: DiscoveryMagic}
	got, ok := protocolDialectForMessage(message)
	if !ok || got.Name != ProtocolName || got.Major != ProtocolMajor || got.Magic != DiscoveryMagic {
		t.Fatalf("canonical dialect was not accepted: %+v, %v", got, ok)
	}
	for _, message := range []wireMessage{
		{Protocol: "unknown", Major: 2, Magic: DiscoveryMagic},
		{Protocol: "FlyQPro", Major: 2, Magic: "FLYQPRO_DISCOVERY_V1"},
		{Protocol: "POPChat", Major: 1, Magic: "POPCHAT_DISCOVERY_V1"},
		{Protocol: ProtocolName, Major: ProtocolMajor, Magic: "WRONG_MAGIC"},
	} {
		if _, ok := protocolDialectForMessage(message); ok {
			t.Fatalf("unsupported dialect tuple was accepted: %+v", message)
		}
	}
}

func TestProtocolPeerWithUnknownDialectUsesCanonical(t *testing.T) {
	for _, peer := range []Peer{
		{ProtocolName: "FlyQPro", ProtocolMajor: 2, DiscoveryMagic: "FLYQPRO_DISCOVERY_V1"},
		{ProtocolName: "POPChat", ProtocolMajor: 1, DiscoveryMagic: "POPCHAT_DISCOVERY_V1"},
		{},
	} {
		got := protocolDialectsForPeer(peer)
		if len(got) != 1 || got[0].Name != ProtocolName {
			t.Fatalf("peer should use canonical dialect: %+v", got)
		}
	}
}

func TestHelloMessageUsesCanonicalProtocol(t *testing.T) {
	engine := NewEngine()
	dialect := protocolDialects[0]
	message := engine.helloMessageForDialect("hello", dialect)
	if message.Protocol != ProtocolName || message.Major != ProtocolMajor || message.Magic != DiscoveryMagic {
		t.Fatalf("hello did not use canonical dialect: %+v", message)
	}
	for _, capability := range []string{"text", "image", "file", "file-progress-v1", "avatar-sync-v1", "offline-v1", "friend-restore-v2"} {
		if !hasCapability(message.Capabilities, capability) {
			t.Fatalf("capability %q missing: %v", capability, message.Capabilities)
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

func TestDiscoveryPermissionAllowsFriendsWhenDisabled(t *testing.T) {
	engine := NewEngine()
	engine.profile = Profile{Discoverable: false}
	engine.peers["friend-1"] = Peer{DeviceID: "friend-1", Relation: PeerRelation}

	if !engine.canRespondToDiscovery("friend-1") {
		t.Fatal("friends must remain discoverable when general discovery is disabled")
	}
	if engine.canRespondToDiscovery("stranger-1") {
		t.Fatal("strangers must not be discoverable when general discovery is disabled")
	}
	if !engine.canAcceptPeerConnection("friend-1") {
		t.Fatal("friends must remain reachable when general discovery is disabled")
	}
}

func TestDiscoveryPermissionAllowsStrangersWhenEnabled(t *testing.T) {
	engine := NewEngine()
	engine.profile = Profile{Discoverable: true}

	if !engine.canRespondToDiscovery("stranger-1") {
		t.Fatal("strangers must be discoverable when general discovery is enabled")
	}
	if !engine.canAcceptPeerConnection("stranger-1") {
		t.Fatal("discoverable devices must accept stranger connections")
	}
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
