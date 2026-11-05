// Copyright (C) 2024  mieru authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// Package mieruclient provides C-exported bindings for the mieru proxy client.
//
// This file holds all pure-Go logic (no import "C") so it can be compiled and
// checked with CGO_ENABLED=0. The cgo entry points live in main.go and only
// marshal C arguments into this layer.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enfein/mieru/v3/apis/client"
	"github.com/enfein/mieru/v3/apis/model"
	"github.com/enfein/mieru/v3/pkg/appctl/appctlpb"
	"github.com/enfein/mieru/v3/pkg/log"
	"google.golang.org/protobuf/proto"
)

// mieruDevice wraps a single mieru client instance. A device can be used only
// once: after Stop() the underlying client can't be reused, so Java frees the
// handle and creates a new one for the next proxy session.
type mieruDevice struct {
	mu         sync.Mutex
	client     client.Client
	configured bool
	running    bool
}

var (
	handlesMu sync.Mutex
	handles         = make(map[int64]*mieruDevice)
	nextID    int64 = 1

	activeMu         sync.Mutex
	activeHandle     int64 = 0
	activeGeneration uint64

	// outstanding asynchronous dials keyed by monotonically increasing id.
	dialsMu  sync.Mutex
	dials           = map[uint64]*pendingDial{}
	dialNext uint64 = 1
)

type pendingDial struct {
	ctx    context.Context
	cancel context.CancelFunc
	client client.Client
}

// The hook functions are installed from main.go (cgo layer) and call back into
// libtmessages.so. They are kept behind a mutex so that a CGO_ENABLED=0 build
// of this file remains compilable.
var (
	hookMu        sync.RWMutex
	connectedHook func(instanceNum int32, token uint64, dialID uint64, fd int32)
	failedHook    func(instanceNum int32, token uint64, dialID uint64, errorCode int32)
)

func setHooks(connected func(int32, uint64, uint64, int32), failed func(int32, uint64, uint64, int32)) {
	hookMu.Lock()
	connectedHook = connected
	failedHook = failed
	hookMu.Unlock()
}

func notifyDialConnected(instanceNum int32, token uint64, dialID uint64, fd int32) {
	hookMu.RLock()
	h := connectedHook
	hookMu.RUnlock()
	if h != nil {
		h(instanceNum, token, dialID, fd)
	}
}

func notifyDialFailed(instanceNum int32, token uint64, dialID uint64) {
	hookMu.RLock()
	h := failedHook
	hookMu.RUnlock()
	if h != nil {
		h(instanceNum, token, dialID, -1)
	}
}

// appNewClient creates a new mieru client instance and returns its handle.
func appNewClient() int64 {
	dev := &mieruDevice{client: client.NewClient()}
	handlesMu.Lock()
	id := nextID
	nextID++
	handles[id] = dev
	handlesMu.Unlock()
	return id
}

func getDevice(handle int64) *mieruDevice {
	handlesMu.Lock()
	dev, ok := handles[handle]
	handlesMu.Unlock()
	if !ok {
		return nil
	}
	return dev
}

// appFreeClient destroys a client instance.
func appFreeClient(handle int64) {
	handlesMu.Lock()
	dev, ok := handles[handle]
	if ok {
		delete(handles, handle)
	}
	handlesMu.Unlock()
	if dev == nil {
		return
	}
	dev.mu.Lock()
	if dev.client != nil && dev.running {
		dev.client.Stop()
	}
	dev.mu.Unlock()
	activeMu.Lock()
	if activeHandle == handle {
		activeHandle = 0
		activeGeneration++
	}
	activeMu.Unlock()
	cancelAllDials()
}

// appConfigure stores the validated client profile.
func appConfigure(handle int64, server string, portSpec string, username string, password string, mtu int32, protocolStr string) bool {
	dev := getDevice(handle)
	if dev == nil {
		return false
	}
	if server == "" || portSpec == "" {
		return false
	}
	if username == "" || password == "" {
		return false
	}
	if mtu == 0 {
		mtu = 1400
	}
	if mtu < 1280 || mtu > 1500 {
		return false
	}
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.running {
		return false
	}
	profile, err := buildProfile(server, portSpec, username, password, mtu, protocolStr)
	if err != nil {
		log.Errorf("mieru configure failed: %v", err)
		return false
	}
	if err := dev.client.Store(&client.ClientConfig{Profile: profile}); err != nil {
		log.Errorf("mieru configure failed: %v", err)
		return false
	}
	dev.configured = true
	return true
}

func buildProfile(server string, portSpec string, username string, password string, mtu int32, protocolStr string) (*appctlpb.ClientProfile, error) {
	portBindings, err := parsePortBindings(portSpec, protocolStr)
	if err != nil {
		return nil, err
	}
	serverEndpoint := &appctlpb.ServerEndpoint{PortBindings: portBindings}
	if net.ParseIP(server) != nil {
		serverEndpoint.IpAddress = proto.String(server)
	} else {
		serverEndpoint.DomainName = proto.String(server)
	}
	profile := &appctlpb.ClientProfile{
		ProfileName: proto.String("telegram"),
		User: &appctlpb.User{
			Name:     proto.String(username),
			Password: proto.String(password),
		},
		Servers: []*appctlpb.ServerEndpoint{serverEndpoint},
		Mtu:     proto.Int32(mtu),
		Multiplexing: &appctlpb.MultiplexingConfig{
			Level: appctlpb.MultiplexingLevel_MULTIPLEXING_LOW.Enum(),
		},
		HandshakeMode: appctlpb.HandshakeMode_HANDSHAKE_STANDARD.Enum(),
	}
	return profile, nil
}

func parsePortBindings(portSpec string, protocolStr string) ([]*appctlpb.PortBinding, error) {
	protocols := []string{protocolStr}
	if protocolStr == "TCP+UDP" {
		protocols = []string{"TCP", "UDP"}
	}
	bindings := make([]*appctlpb.PortBinding, 0, len(protocols))
	for _, protocol := range protocols {
		binding, err := parsePortBinding(portSpec, protocol)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	return bindings, nil
}

func parsePortBinding(portSpec string, protocolStr string) (*appctlpb.PortBinding, error) {
	var transportProto appctlpb.TransportProtocol
	switch protocolStr {
	case "TCP":
		transportProto = appctlpb.TransportProtocol_TCP
	case "UDP":
		transportProto = appctlpb.TransportProtocol_UDP
	default:
		return nil, fmt.Errorf("unsupported transport protocol %q", protocolStr)
	}
	binding := &appctlpb.PortBinding{Protocol: &transportProto}
	if port, err := strconv.Atoi(portSpec); err == nil {
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("port %d is out of range", port)
		}
		binding.Port = proto.Int32(int32(port))
		return binding, nil
	}
	parts := strings.Split(portSpec, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid port or port range %q", portSpec)
	}
	first, firstErr := strconv.Atoi(parts[0])
	last, lastErr := strconv.Atoi(parts[1])
	if firstErr != nil || lastErr != nil || first < 1 || last > 65535 || first > last {
		return nil, fmt.Errorf("invalid port range %q", portSpec)
	}
	binding.PortRange = proto.String(fmt.Sprintf("%d-%d", first, last))
	return binding, nil
}

// appStart activates the client.
func appStart(handle int64) bool {
	dev := getDevice(handle)
	if dev == nil {
		return false
	}
	dev.mu.Lock()
	defer dev.mu.Unlock()
	if dev.running {
		return true
	}
	if !dev.configured {
		return false
	}
	if err := dev.client.Start(); err != nil {
		log.Errorf("mieru start failed: %v", err)
		return false
	}
	dev.running = true
	return true
}

// appStop deactivates the client.
func appStop(handle int64) {
	dev := getDevice(handle)
	if dev == nil {
		return
	}
	dev.mu.Lock()
	if dev.running {
		dev.client.Stop()
		dev.running = false
	}
	dev.mu.Unlock()
	cancelAllDials()
}

func appIsRunning(handle int64) bool {
	dev := getDevice(handle)
	if dev == nil {
		return false
	}
	dev.mu.Lock()
	running := dev.running
	dev.mu.Unlock()
	return running
}

// appActivate makes the client the target for tgnet ConnectionSocket dials.
func appActivate(handle int64) bool {
	dev := getDevice(handle)
	if dev == nil {
		return false
	}
	dev.mu.Lock()
	running := dev.running
	dev.mu.Unlock()
	if !running {
		return false
	}
	activeMu.Lock()
	activeHandle = handle
	activeGeneration++
	activeMu.Unlock()
	return true
}

func appDeactivate() {
	activeMu.Lock()
	activeHandle = 0
	activeGeneration++
	activeMu.Unlock()
	cancelAllDials()
}

// appDial starts an asynchronous connection attempt. token is an opaque C++
// callback-registry id returned verbatim in the connected/failed callbacks.
// Returns the dial id (for cancel) and whether a dial was started.
func appDial(instanceNum int32, token uint64, address string, port int, timeoutMillis int32) (uint64, bool) {
	activeMu.Lock()
	handle := activeHandle
	generation := activeGeneration
	activeMu.Unlock()
	if handle == 0 {
		return 0, false
	}
	dev := getDevice(handle)
	if dev == nil {
		return 0, false
	}
	dev.mu.Lock()
	cl := dev.client
	running := dev.running
	dev.mu.Unlock()
	if cl == nil || !running {
		return 0, false
	}
	if address == "" || port <= 0 || port > 65535 {
		return 0, false
	}
	timeout := time.Duration(timeoutMillis) * time.Millisecond
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	activeMu.Lock()
	if activeHandle != handle || activeGeneration != generation {
		activeMu.Unlock()
		cancel()
		return 0, false
	}
	dialsMu.Lock()
	dialID := dialNext
	dialNext++
	dials[dialID] = &pendingDial{ctx: ctx, cancel: cancel, client: cl}
	dialsMu.Unlock()
	activeMu.Unlock()

	go dialWorker(dialID, instanceNum, token, cl, ctx, address, port)
	return dialID, true
}

// appCancelDial aborts an in-flight dial. The dial worker detects the
// cancelled state and cleans up without delivering any callback.
func appCancelDial(dialID uint64) {
	dialsMu.Lock()
	p, ok := dials[dialID]
	if ok {
		delete(dials, dialID)
	}
	dialsMu.Unlock()
	if ok && p != nil {
		p.cancel()
	}
}

func cancelAllDials() {
	dialsMu.Lock()
	for _, p := range dials {
		p.cancel()
	}
	dials = make(map[uint64]*pendingDial)
	dialsMu.Unlock()
}

func dialWorker(dialID uint64, instanceNum int32, token uint64, cl client.Client, ctx context.Context, address string, port int) {
	conn := dialOne(cl, ctx, address, port)

	// Re-check the pending table. If the dial was cancelled concurrently the
	// entry is gone and we must not deliver any callback.
	dialsMu.Lock()
	_, ok := dials[dialID]
	if ok {
		delete(dials, dialID)
	}
	dialsMu.Unlock()
	if !ok {
		if conn != nil {
			conn.Close()
		}
		return
	}
	if conn == nil {
		notifyDialFailed(instanceNum, token, dialID)
		return
	}
	fd, setupOK := setupPair(conn)
	if !setupOK {
		conn.Close()
		notifyDialFailed(instanceNum, token, dialID)
		return
	}
	notifyDialConnected(instanceNum, token, dialID, fd)
}

func dialOne(cl client.Client, ctx context.Context, address string, port int) net.Conn {
	var target net.Addr
	if ip := net.ParseIP(address); ip != nil {
		target = &net.TCPAddr{IP: ip, Port: port}
	} else {
		target = &model.NetAddrSpec{
			Net: "tcp",
			AddrSpec: model.AddrSpec{
				FQDN: address,
				Port: port,
			},
		}
	}
	conn, err := cl.DialContext(ctx, target)
	if err != nil {
		return nil
	}
	return conn
}

type sharedConfig struct {
	ServerAddress string `json:"server"`
	PortSpec      string `json:"port"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	MTU           int32  `json:"mtu"`
	Protocol      string `json:"protocol"`
}

func appParseURL(rawURL string) string {
	profile, err := profileFromURL(rawURL)
	if err != nil {
		log.Errorf("mieru URL parse failed: %v", err)
		return ""
	}
	config, err := sharedConfigFromProfile(profile)
	if err != nil {
		log.Errorf("mieru URL conversion failed: %v", err)
		return ""
	}
	b, err := json.Marshal(config)
	if err != nil {
		return ""
	}
	return string(b)
}

func appCreateURL(server string, portSpec string, username string, password string, mtu int32, protocolStr string) string {
	profile, err := buildProfile(server, portSpec, username, password, mtu, protocolStr)
	if err != nil {
		return ""
	}
	serverEndpoint := profile.GetServers()[0]
	u := &url.URL{Scheme: "mierus", User: url.UserPassword(username, password)}
	if serverEndpoint.GetDomainName() != "" {
		u.Host = serverEndpoint.GetDomainName()
	} else {
		u.Host = serverEndpoint.GetIpAddress()
		if strings.Contains(u.Host, ":") {
			u.Host = "[" + u.Host + "]"
		}
	}
	q := url.Values{}
	q.Add("profile", profile.GetProfileName())
	q.Add("mtu", strconv.Itoa(int(profile.GetMtu())))
	q.Add("multiplexing", profile.GetMultiplexing().GetLevel().String())
	q.Add("handshake-mode", profile.GetHandshakeMode().String())
	for _, binding := range serverEndpoint.GetPortBindings() {
		q.Add("port", portSpecFromBinding(binding))
		q.Add("protocol", binding.GetProtocol().String())
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func profileFromURL(rawURL string) (*appctlpb.ClientProfile, error) {
	if strings.HasPrefix(rawURL, "mieru://") {
		data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(rawURL, "mieru://"))
		if err != nil {
			return nil, err
		}
		config := &appctlpb.ClientConfig{}
		if err := proto.Unmarshal(data, config); err != nil {
			return nil, err
		}
		for _, profile := range config.GetProfiles() {
			if profile.GetProfileName() == config.GetActiveProfile() {
				return profile, nil
			}
		}
		return nil, fmt.Errorf("active profile %q is not present", config.GetActiveProfile())
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "mierus" || u.User == nil || u.Hostname() == "" {
		return nil, fmt.Errorf("invalid mierus URL")
	}
	password, hasPassword := u.User.Password()
	if u.User.Username() == "" || !hasPassword || password == "" {
		return nil, fmt.Errorf("missing credentials")
	}
	q := u.Query()
	ports, protocols := q["port"], q["protocol"]
	if len(ports) != len(protocols) || len(ports) < 1 || len(ports) > 2 {
		return nil, fmt.Errorf("Telegram supports one port binding or a TCP+UDP pair per Mieru profile")
	}
	portSpec := ports[0]
	protocolStr := protocols[0]
	if len(ports) == 2 {
		if ports[1] != portSpec || !isTCPUDPPair(protocols[0], protocols[1]) {
			return nil, fmt.Errorf("Telegram TCP+UDP profiles require matching TCP and UDP port bindings")
		}
		protocolStr = "TCP+UDP"
	}
	mtu := int32(1400)
	if q.Get("mtu") != "" {
		parsed, err := strconv.Atoi(q.Get("mtu"))
		if err != nil {
			return nil, fmt.Errorf("invalid MTU %q", q.Get("mtu"))
		}
		mtu = int32(parsed)
	}
	profile, err := buildProfile(u.Hostname(), portSpec, u.User.Username(), password, mtu, protocolStr)
	if err != nil {
		return nil, err
	}
	if q.Get("profile") != "" {
		profile.ProfileName = proto.String(q.Get("profile"))
	}
	return profile, nil
}

func sharedConfigFromProfile(profile *appctlpb.ClientProfile) (*sharedConfig, error) {
	if profile == nil || len(profile.GetServers()) != 1 {
		return nil, fmt.Errorf("Telegram supports exactly one server")
	}
	server := profile.GetServers()[0]
	bindings := server.GetPortBindings()
	if len(bindings) < 1 || len(bindings) > 2 {
		return nil, fmt.Errorf("Telegram supports one port binding or a TCP+UDP pair")
	}
	address := server.GetDomainName()
	if address == "" {
		address = server.GetIpAddress()
	}
	portSpec := portSpecFromBinding(bindings[0])
	protocolStr := bindings[0].GetProtocol().String()
	if _, err := parsePortBinding(portSpec, protocolStr); err != nil {
		return nil, err
	}
	if len(bindings) == 2 {
		secondPortSpec := portSpecFromBinding(bindings[1])
		secondProtocol := bindings[1].GetProtocol().String()
		if secondPortSpec != portSpec || !isTCPUDPPair(protocolStr, secondProtocol) {
			return nil, fmt.Errorf("Telegram TCP+UDP profiles require matching TCP and UDP port bindings")
		}
		protocolStr = "TCP+UDP"
	}
	mtu := profile.GetMtu()
	if mtu == 0 {
		mtu = 1400
	}
	if mtu < 1280 || mtu > 1500 {
		return nil, fmt.Errorf("MTU %d is out of range", mtu)
	}
	user := profile.GetUser()
	if address == "" || user.GetName() == "" || user.GetPassword() == "" {
		return nil, fmt.Errorf("profile is missing required fields")
	}
	return &sharedConfig{
		ServerAddress: address,
		PortSpec:      portSpec,
		Username:      user.GetName(),
		Password:      user.GetPassword(),
		MTU:           mtu,
		Protocol:      protocolStr,
	}, nil
}

func portSpecFromBinding(binding *appctlpb.PortBinding) string {
	if binding.GetPortRange() != "" {
		return binding.GetPortRange()
	}
	return strconv.Itoa(int(binding.GetPort()))
}

func isTCPUDPPair(first string, second string) bool {
	return first == "TCP" && second == "UDP" || first == "UDP" && second == "TCP"
}

func main() {
	// No-op main is required for -buildmode=c-shared and keeps the pure-Go
	// package valid under CGO_ENABLED=0 builds used for compile checks.
}
