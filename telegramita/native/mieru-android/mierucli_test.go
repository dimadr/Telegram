package main

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/enfein/mieru/v3/pkg/appctl/appctlpb"
	"google.golang.org/protobuf/proto"
)

func TestBuildProfileAddressAndPortModels(t *testing.T) {
	tests := []struct {
		name       string
		address    string
		port       string
		protocol   string
		wantIP     string
		wantDomain string
		wantPort   int32
		wantRange  string
	}{
		{name: "IPv4 single TCP", address: "192.0.2.1", port: "7070", protocol: "TCP", wantIP: "192.0.2.1", wantPort: 7070},
		{name: "domain range UDP", address: "example.com", port: "7070-7079", protocol: "UDP", wantDomain: "example.com", wantRange: "7070-7079"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := buildProfile(tt.address, tt.port, "user", "password", 1400, tt.protocol)
			if err != nil {
				t.Fatalf("buildProfile() error = %v", err)
			}
			server := profile.GetServers()[0]
			binding := server.GetPortBindings()[0]
			if server.GetIpAddress() != tt.wantIP || server.GetDomainName() != tt.wantDomain || binding.GetPort() != tt.wantPort || binding.GetPortRange() != tt.wantRange || binding.GetProtocol().String() != tt.protocol {
				t.Fatalf("unexpected endpoint: %v", server)
			}
		})
	}
}

func TestRejectInvalidConfiguration(t *testing.T) {
	tests := []struct {
		port     string
		protocol string
	}{
		{port: "0", protocol: "TCP"},
		{port: "7079-7070", protocol: "TCP"},
		{port: "7070", protocol: "tcp"},
		{port: "7070", protocol: "QUIC"},
	}
	for _, tt := range tests {
		if _, err := buildProfile("example.com", tt.port, "user", "password", 1400, tt.protocol); err == nil {
			t.Fatalf("buildProfile(%q, %q) unexpectedly succeeded", tt.port, tt.protocol)
		}
	}
}

func TestBuildProfileCombinedTransport(t *testing.T) {
	profile, err := buildProfile("example.com", "7070-7079", "user", "password", 1400, "TCP+UDP")
	if err != nil {
		t.Fatal(err)
	}
	bindings := profile.GetServers()[0].GetPortBindings()
	if len(bindings) != 2 {
		t.Fatalf("got %d port bindings, want 2", len(bindings))
	}
	if bindings[0].GetProtocol().String() != "TCP" || bindings[1].GetProtocol().String() != "UDP" {
		t.Fatalf("unexpected protocols: %s, %s", bindings[0].GetProtocol(), bindings[1].GetProtocol())
	}
	for _, binding := range bindings {
		if binding.GetPortRange() != "7070-7079" {
			t.Fatalf("port range = %q, want 7070-7079", binding.GetPortRange())
		}
	}
}

func TestSimpleURLRoundTrip(t *testing.T) {
	created := appCreateURL("example.com", "4012-4021", "user:name", "p@ss word", 1400, "TCP")
	if created == "" {
		t.Fatal("appCreateURL() returned empty string")
	}
	parsed := appParseURL(created)
	want := `{"server":"example.com","port":"4012-4021","username":"user:name","password":"p@ss word","mtu":1400,"protocol":"TCP"}`
	if parsed != want {
		t.Fatalf("round trip = %s, want %s; URL = %s", parsed, want, created)
	}
}

func TestSimpleURLIPv6RoundTrip(t *testing.T) {
	created := appCreateURL("2001:db8::1", "443", "user", "password", 1400, "TCP")
	if !strings.Contains(created, "@[2001:db8::1]") {
		t.Fatalf("IPv6 host is not bracketed: %q", created)
	}
	parsed := appParseURL(created)
	var got sharedConfig
	if err := json.Unmarshal([]byte(parsed), &got); err != nil {
		t.Fatalf("json.Unmarshal() error: %v", err)
	}
	if got.ServerAddress != "2001:db8::1" || got.PortSpec != "443" {
		t.Fatalf("unexpected parsed IPv6 config: %+v", got)
	}
}

func TestCombinedTransportURLRoundTrip(t *testing.T) {
	created := appCreateURL("example.com", "4012-4021", "user", "password", 1400, "TCP+UDP")
	if created == "" {
		t.Fatal("appCreateURL() returned empty string")
	}
	parsed := appParseURL(created)
	want := `{"server":"example.com","port":"4012-4021","username":"user","password":"password","mtu":1400,"protocol":"TCP+UDP"}`
	if parsed != want {
		t.Fatalf("round trip = %s, want %s; URL = %s", parsed, want, created)
	}
}

func TestStandardURLParsing(t *testing.T) {
	profile, err := buildProfile("2001:db8::1", "7070", "user", "password", 1400, "UDP")
	if err != nil {
		t.Fatal(err)
	}
	config := &appctlpb.ClientConfig{
		Profiles:      []*appctlpb.ClientProfile{profile},
		ActiveProfile: proto.String(profile.GetProfileName()),
	}
	b, err := proto.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	parsed := appParseURL("mieru://" + base64.StdEncoding.EncodeToString(b))
	want := `{"server":"2001:db8::1","port":"7070","username":"user","password":"password","mtu":1400,"protocol":"UDP"}`
	if parsed != want {
		t.Fatalf("parsed = %s, want %s", parsed, want)
	}
}
