package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"net"
	"net/netip"
	"testing"

	"github.com/database64128/swgp-go/conn"
	"github.com/database64128/swgp-go/packet"
	"go.uber.org/zap"
)

var logger *zap.Logger

func generateTestPSK(t *testing.T) []byte {
	psk := make([]byte, 32)
	_, err := rand.Read(psk)
	if err != nil {
		t.Fatal(err)
	}
	return psk
}

func testClientServerHandshake(t *testing.T, ctx context.Context, serverConfig ServerConfig, clientConfig ClientConfig) {
	sc := Config{
		Servers: []ServerConfig{serverConfig},
		Clients: []ClientConfig{clientConfig},
	}
	m, err := sc.Manager(logger)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	// Make packets.
	handshakeInitiationPacket := make([]byte, packet.WireGuardMessageLengthHandshakeInitiation)
	handshakeInitiationPacket[0] = packet.WireGuardMessageTypeHandshakeInitiation
	if _, err = rand.Read(handshakeInitiationPacket[1:]); err != nil {
		t.Fatal(err)
	}
	expectedHandshakeInitiationPacket := make([]byte, packet.WireGuardMessageLengthHandshakeInitiation)
	copy(expectedHandshakeInitiationPacket, handshakeInitiationPacket)
	receivedHandshakeInitiationPacket := make([]byte, packet.WireGuardMessageLengthHandshakeInitiation+1)

	handshakeResponsePacket := make([]byte, packet.WireGuardMessageLengthHandshakeResponse)
	handshakeResponsePacket[0] = packet.WireGuardMessageTypeHandshakeResponse
	if _, err = rand.Read(handshakeResponsePacket[1:]); err != nil {
		t.Fatal(err)
	}
	expectedHandshakeResponsePacket := make([]byte, packet.WireGuardMessageLengthHandshakeResponse)
	copy(expectedHandshakeResponsePacket, handshakeResponsePacket)
	receivedHandshakeResponsePacket := make([]byte, packet.WireGuardMessageLengthHandshakeResponse+1)

	// Start client and server conns.
	clientConn, err := net.Dial("udp", clientConfig.WgListen)
	if err != nil {
		t.Fatal(err)
	}
	serverConn, err := conn.DefaultUDPClientListenConfig.ListenUDP(ctx, "udp", serverConfig.WgEndpoint.String())
	if err != nil {
		t.Fatal(err)
	}

	// Client sends handshake initiation.
	_, err = clientConn.Write(handshakeInitiationPacket)
	if err != nil {
		t.Fatal(err)
	}

	// Server receives handshake initiation.
	n, addr, err := serverConn.ReadFromUDPAddrPort(receivedHandshakeInitiationPacket)
	if err != nil {
		t.Fatal(err)
	}

	// Server verifies handshake initiation.
	if !bytes.Equal(receivedHandshakeInitiationPacket[:n], expectedHandshakeInitiationPacket) {
		t.Error("Received handshake initiation packet does not match expectation.")
	}

	// Server sends handshake response.
	_, err = serverConn.WriteToUDPAddrPort(handshakeResponsePacket, addr)
	if err != nil {
		t.Fatal(err)
	}

	// Client receives handshake response.
	n, err = clientConn.Read(receivedHandshakeResponsePacket)
	if err != nil {
		t.Fatal(err)
	}

	// Client verifies handshake response.
	if !bytes.Equal(receivedHandshakeResponsePacket[:n], expectedHandshakeResponsePacket) {
		t.Error("Received handshake response packet does not match expectation.")
	}
}

func TestClientServerHandshakeZeroOverhead(t *testing.T) {
	psk := generateTestPSK(t)

	serverConfig := ServerConfig{
		Name:        "wg0",
		ProxyListen: ":20220",
		ProxyMode:   "zero-overhead",
		ProxyPSK:    psk,
		WgEndpoint:  conn.AddrFromIPPort(netip.AddrPortFrom(netip.IPv6Loopback(), 20221)),
		MTU:         1500,
	}

	clientConfig := ClientConfig{
		Name:          "wg0",
		WgListen:      ":20222",
		ProxyEndpoint: conn.AddrFromIPPort(netip.AddrPortFrom(netip.IPv6Loopback(), 20220)),
		ProxyMode:     "zero-overhead",
		ProxyPSK:      psk,
		MTU:           1500,
	}

	testClientServerHandshake(t, context.Background(), serverConfig, clientConfig)
}

func TestClientServerHandshakeParanoid(t *testing.T) {
	psk := generateTestPSK(t)

	serverConfig := ServerConfig{
		Name:        "wg0",
		ProxyListen: ":20223",
		ProxyMode:   "paranoid",
		ProxyPSK:    psk,
		WgEndpoint:  conn.AddrFromIPPort(netip.AddrPortFrom(netip.IPv6Loopback(), 20224)),
		MTU:         1500,
	}

	clientConfig := ClientConfig{
		Name:          "wg0",
		WgListen:      ":20225",
		ProxyEndpoint: conn.AddrFromIPPort(netip.AddrPortFrom(netip.IPv6Loopback(), 20223)),
		ProxyMode:     "paranoid",
		ProxyPSK:      psk,
		MTU:           1500,
	}

	testClientServerHandshake(t, context.Background(), serverConfig, clientConfig)
}

func testClientServerDataPackets(t *testing.T, ctx context.Context, serverConfig ServerConfig, clientConfig ClientConfig) {
	sc := Config{
		Servers: []ServerConfig{serverConfig},
		Clients: []ClientConfig{clientConfig},
	}
	m, err := sc.Manager(logger)
	if err != nil {
		t.Fatal(err)
	}
	if err = m.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer m.Stop()

	// Make packets.
	smallDataPacket := make([]byte, 1024)
	smallDataPacket[0] = packet.WireGuardMessageTypeData
	if _, err = rand.Read(smallDataPacket[1:]); err != nil {
		t.Fatal(err)
	}
	expectedSmallDataPacket := make([]byte, 1024)
	copy(expectedSmallDataPacket, smallDataPacket)
	receivedSmallDataPacket := make([]byte, 1024+1)

	bigDataPacket := make([]byte, 2048)
	bigDataPacket[0] = packet.WireGuardMessageTypeData
	if _, err = rand.Read(bigDataPacket[1:]); err != nil {
		t.Fatal(err)
	}

	// Start client and server conns.
	clientConn, err := net.Dial("udp", clientConfig.WgListen)
	if err != nil {
		t.Fatal(err)
	}
	serverConn, err := conn.DefaultUDPClientListenConfig.ListenUDP(ctx, "udp", serverConfig.WgEndpoint.String())
	if err != nil {
		t.Fatal(err)
	}

	// Client sends big data packet.
	_, err = clientConn.Write(bigDataPacket)
	if err != nil {
		t.Fatal(err)
	}

	// Client sends small data packet.
	_, err = clientConn.Write(smallDataPacket)
	if err != nil {
		t.Fatal(err)
	}

	// Server receives small data packet.
	n, addr, err := serverConn.ReadFromUDPAddrPort(receivedSmallDataPacket)
	if err != nil {
		t.Fatal(err)
	}

	// Server verifies small data packet.
	if !bytes.Equal(receivedSmallDataPacket[:n], expectedSmallDataPacket) {
		t.Error("Received small data packet does not match expectation.")
	}

	// Server sends big data packet.
	_, err = serverConn.WriteToUDPAddrPort(bigDataPacket, addr)
	if err != nil {
		t.Fatal(err)
	}

	// Server sends small data packet.
	_, err = serverConn.WriteToUDPAddrPort(smallDataPacket, addr)
	if err != nil {
		t.Fatal(err)
	}

	// Client receives small data packet.
	n, err = clientConn.Read(receivedSmallDataPacket)
	if err != nil {
		t.Fatal(err)
	}

	// Client verifies small data packet.
	if !bytes.Equal(receivedSmallDataPacket[:n], expectedSmallDataPacket) {
		t.Error("Received small data packet does not match expectation.")
	}
}

func TestClientServerDataPacketsZeroOverhead(t *testing.T) {
	psk := generateTestPSK(t)

	serverConfig := ServerConfig{
		Name:        "wg0",
		ProxyListen: ":20230",
		ProxyMode:   "zero-overhead",
		ProxyPSK:    psk,
		WgEndpoint:  conn.AddrFromIPPort(netip.AddrPortFrom(netip.IPv6Loopback(), 20231)),
		MTU:         1500,
	}

	clientConfig := ClientConfig{
		Name:          "wg0",
		WgListen:      ":20232",
		ProxyEndpoint: conn.AddrFromIPPort(netip.AddrPortFrom(netip.IPv6Loopback(), 20230)),
		ProxyMode:     "zero-overhead",
		ProxyPSK:      psk,
		MTU:           1500,
	}

	testClientServerDataPackets(t, context.Background(), serverConfig, clientConfig)
}

func TestClientServerDataPacketsParanoid(t *testing.T) {
	psk := generateTestPSK(t)

	serverConfig := ServerConfig{
		Name:        "wg0",
		ProxyListen: ":20233",
		ProxyMode:   "paranoid",
		ProxyPSK:    psk,
		WgEndpoint:  conn.AddrFromIPPort(netip.AddrPortFrom(netip.IPv6Loopback(), 20234)),
		MTU:         1500,
	}

	clientConfig := ClientConfig{
		Name:          "wg0",
		WgListen:      ":20235",
		ProxyEndpoint: conn.AddrFromIPPort(netip.AddrPortFrom(netip.IPv6Loopback(), 20233)),
		ProxyMode:     "paranoid",
		ProxyPSK:      psk,
		MTU:           1500,
	}

	testClientServerDataPackets(t, context.Background(), serverConfig, clientConfig)
}

func TestMain(m *testing.M) {
	var err error
	logger, err = zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	m.Run()
}
