package chat

import (
	"encoding/json"
	"fmt"
)

const v3FinalizationErrorVersion = 1

type v3FinalizationError struct {
	Version       int               `json:"version"`
	Status        string            `json:"status"`
	ErrorCode     TransferErrorCode `json:"errorCode"`
	Retryable     bool              `json:"retryable"`
	Verified      bool              `json:"verified"`
	DurableBytes  int64             `json:"durableBytes"`
	CommittedPath string            `json:"committedPath,omitempty"`
}

func encodeV3FinalizationError(result v3FinalizationError) ([]byte, error) {
	result.Version = v3FinalizationErrorVersion
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if len(data) > 1024 {
		return nil, fmt.Errorf("v3 finalization error payload too large")
	}
	return data, nil
}

func decodeV3FinalizationError(data []byte) (v3FinalizationError, error) {
	var result v3FinalizationError
	if len(data) == 0 || len(data) > 1024 {
		return result, fmt.Errorf("invalid v3 finalization error payload")
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	if result.Version != v3FinalizationErrorVersion || result.Status == "" || result.ErrorCode == "" {
		return result, fmt.Errorf("invalid v3 finalization error payload")
	}
	return result, nil
}
