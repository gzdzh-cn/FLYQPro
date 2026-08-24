package chat

import (
	"encoding/json"
	"testing"
)

func TestRequiredAttachmentBytesAddsSafetyMargin(t *testing.T) {
	if got, want := requiredAttachmentBytes(100), int64(100)+attachmentSafetyMargin; got != want {
		t.Fatalf("required bytes = %d, want %d", got, want)
	}
	if got := requiredAttachmentBytes(-1); got <= 0 {
		t.Fatalf("negative file size should remain rejectable, got %d", got)
	}
}

func TestFileOfferResponseWireFieldsArePortable(t *testing.T) {
	input := wireMessage{Type: "file_offer_response", AttachmentID: "a", Status: "rejected", Reason: "INSUFFICIENT_STORAGE", AvailableBytes: 12, RequiredBytes: 34}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output wireMessage
	if err := json.Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if output.Reason != input.Reason || output.AvailableBytes != input.AvailableBytes || output.RequiredBytes != input.RequiredBytes {
		t.Fatalf("wire response fields did not round-trip: %+v", output)
	}
}
