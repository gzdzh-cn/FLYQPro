package chat

import "testing"

func TestActiveFriendRequestStatus(t *testing.T) {
	for _, status := range []string{"queued", "sent", "pending"} {
		if !isActiveFriendRequest(status) {
			t.Fatalf("%q should be an active request", status)
		}
	}
	for _, status := range []string{"accepted", "rejected", "failed", "superseded"} {
		if isActiveFriendRequest(status) {
			t.Fatalf("%q should be terminal request history", status)
		}
	}
}

func TestFriendRequestIdentityIsRequestID(t *testing.T) {
	first := FriendRequest{RequestID: "old", DeviceID: "peer-1", Status: "accepted"}
	second := FriendRequest{RequestID: "new", DeviceID: "peer-1", Status: "pending"}
	if first.RequestID == second.RequestID || first.DeviceID != second.DeviceID {
		t.Fatal("test setup must represent two requests for one device")
	}
	if isActiveFriendRequest(first.Status) || !isActiveFriendRequest(second.Status) {
		t.Fatal("an old accepted request must not block a new pending request")
	}
}
