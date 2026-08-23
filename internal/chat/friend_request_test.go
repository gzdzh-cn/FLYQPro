package chat

import "testing"

func TestAggregateFriendRequestRowsMergesMutualRequests(t *testing.T) {
	rows := []FriendRequest{
		{RequestID: "outgoing-1", DeviceID: "peer-1", Nickname: "朋友", Message: "我发起", Status: "sent", Direction: "sent", CreatedAt: "2026-08-24T10:00:00Z", UpdatedAt: "2026-08-24T10:00:01Z"},
		{RequestID: "incoming-1", DeviceID: "peer-1", Nickname: "朋友", Message: "对方发起", Status: "pending", Direction: "received", CreatedAt: "2026-08-24T10:02:00Z", UpdatedAt: "2026-08-24T10:02:01Z"},
	}
	merged := aggregateFriendRequestRows(rows)
	if len(merged) != 1 {
		t.Fatalf("双向申请应聚合为一条记录，得到 %d 条", len(merged))
	}
	request := merged[0]
	if request.Direction != "mutual" || request.Status != "pending" || request.RequestID != "incoming-1" {
		t.Fatalf("聚合结果异常: %+v", request)
	}
	if request.CreatedAt != "2026-08-24T10:00:00Z" {
		t.Fatalf("应保留最早申请时间，得到 %q", request.CreatedAt)
	}
}

func TestAggregateFriendRequestRowsKeepsFirstAcceptedTime(t *testing.T) {
	rows := []FriendRequest{
		{RequestID: "outgoing-1", DeviceID: "peer-1", Status: "accepted", Direction: "sent", CreatedAt: "2026-08-24T10:00:00Z", AcceptedAt: "2026-08-24T10:03:00Z", UpdatedAt: "2026-08-24T10:03:00Z"},
		{RequestID: "incoming-1", DeviceID: "peer-1", Status: "accepted", Direction: "received", CreatedAt: "2026-08-24T10:02:00Z", AcceptedAt: "2026-08-24T10:04:00Z", UpdatedAt: "2026-08-24T10:04:00Z"},
	}
	merged := aggregateFriendRequestRows(rows)
	if len(merged) != 1 || merged[0].Status != "accepted" || merged[0].AcceptedAt != "2026-08-24T10:03:00Z" {
		t.Fatalf("应保留首次同意时间，得到 %+v", merged)
	}
}

func TestEarliestAcceptedAtDoesNotMoveOnSecondApproval(t *testing.T) {
	rows := []FriendRequest{
		{RequestID: "outgoing-1", DeviceID: "peer-1", Status: "accepted", AcceptedAt: "2026-08-24T10:03:00Z"},
		{RequestID: "incoming-1", DeviceID: "peer-1", Status: "pending"},
	}
	if acceptedAt := earliestAcceptedAt(rows, "peer-1"); acceptedAt != "2026-08-24T10:03:00Z" {
		t.Fatalf("第二次同意不应覆盖首次时间，得到 %q", acceptedAt)
	}
}

func TestAggregateFriendRequestRowsKeepsSingleDirection(t *testing.T) {
	rows := []FriendRequest{{RequestID: "outgoing-1", DeviceID: "peer-1", Status: "sent", Direction: "sent", CreatedAt: "2026-08-24T10:00:00Z", UpdatedAt: "2026-08-24T10:00:00Z"}}
	merged := aggregateFriendRequestRows(rows)
	if len(merged) != 1 || merged[0].Direction != "sent" || merged[0].RequestID != "outgoing-1" {
		t.Fatalf("单向申请聚合异常: %+v", merged)
	}
}
