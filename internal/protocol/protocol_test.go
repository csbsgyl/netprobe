package protocol

import "testing"

func TestCreateSessionResponseValidate(t *testing.T) {
	valid := CreateSessionResponse{
		Version:   Version,
		SessionID: "session-1",
		Token:     "secret",
		UDPEndpoints: []UDPEndpoint{
			{ID: "primary", Host: "127.0.0.1", Port: 3478},
			{ID: "alternate", Host: "127.0.0.1", Port: 3479},
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid response rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*CreateSessionResponse)
	}{
		{"version", func(r *CreateSessionResponse) { r.Version = 2 }},
		{"session", func(r *CreateSessionResponse) { r.SessionID = "" }},
		{"token", func(r *CreateSessionResponse) { r.Token = "" }},
		{"one endpoint", func(r *CreateSessionResponse) { r.UDPEndpoints = r.UDPEndpoints[:1] }},
		{"three endpoints", func(r *CreateSessionResponse) {
			r.UDPEndpoints = append(r.UDPEndpoints, UDPEndpoint{ID: "third", Host: "127.0.0.1", Port: 3480})
		}},
		{"duplicate id", func(r *CreateSessionResponse) { r.UDPEndpoints[1].ID = "primary" }},
		{"duplicate address", func(r *CreateSessionResponse) { r.UDPEndpoints[1].Port = 3478 }},
		{"bad port", func(r *CreateSessionResponse) { r.UDPEndpoints[1].Port = 70000 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := valid
			got.UDPEndpoints = append([]UDPEndpoint(nil), valid.UDPEndpoints...)
			test.mutate(&got)
			if err := got.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCompleteSessionResponseValidate(t *testing.T) {
	response := CompleteSessionResponse{
		Version:   Version,
		SessionID: "session-1",
		Verdict:   VerdictPass,
		Checks:    []CheckResult{{Name: "udp", Status: CheckPass}},
	}
	if err := response.Validate("session-1"); err != nil {
		t.Fatalf("valid response rejected: %v", err)
	}
	response.Verdict = "maybe"
	if err := response.Validate("session-1"); err == nil {
		t.Fatal("expected invalid verdict error")
	}
}
