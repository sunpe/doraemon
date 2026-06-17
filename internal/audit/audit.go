package audit

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID              string            `json:"id"`
	Timestamp       time.Time         `json:"ts"`
	User            string            `json:"user,omitempty"`
	TokenID         string            `json:"token_id,omitempty"`
	TokenName       string            `json:"token_name,omitempty"`
	Tool            string            `json:"tool,omitempty"`
	Decision        string            `json:"decision"`
	Reason          string            `json:"reason,omitempty"`
	Command         string            `json:"command,omitempty"`
	Args            []string          `json:"args,omitempty"`
	ExitCode        int               `json:"exit_code,omitempty"`
	DurationMS      int64             `json:"duration_ms,omitempty"`
	StdoutBytes     int               `json:"stdout_bytes,omitempty"`
	StderrBytes     int               `json:"stderr_bytes,omitempty"`
	HighRiskAllow   string            `json:"high_risk_allow,omitempty"`
	HighRiskExpires string            `json:"high_risk_expires_at,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

func (e Event) MarshalJSONLine() ([]byte, error) {
	buf, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	return append(buf, '\n'), nil
}
