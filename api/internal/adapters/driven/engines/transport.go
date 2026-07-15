package engines

import (
	"bufio"
	"context"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/huynhanx03/datadock/internal/core/entity"
)

type transport struct {
	dial      func(context.Context, string, string) (net.Conn, error)
	sshClient *ssh.Client
	cleanups  []func()
	closeOnce sync.Once
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (connection *bufferedConn) Read(buffer []byte) (int, error) {
	return connection.reader.Read(buffer)
}

func newTransport(connection entity.Connection) (*transport, error) {
	dial := (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	if connection.ProxyURL != "" {
		proxyDial, err := proxyDialer(connection.ProxyURL, connection.ProxyUsername, connection.ProxyPassword)
		if err != nil {
			return nil, err
		}
		dial = proxyDial
	}
	result := &transport{dial: dial}
	if !connection.SSHTunnel.Enabled {
		return result, nil
	}
	if err := enableSSHTransport(result, connection, dial); err != nil {
		return nil, err
	}
	return result, nil
}

func (transport *transport) Dial(network, address string) (net.Conn, error) {
	return transport.dial(context.Background(), network, address)
}

func (transport *transport) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return transport.dial(ctx, network, address)
}

func (transport *transport) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return transport.dial(ctx, network, address)
}

func (transport *transport) Close() {
	if transport == nil {
		return
	}
	transport.closeOnce.Do(func() {
		if transport.sshClient != nil {
			_ = transport.sshClient.Close()
		}
		for index := len(transport.cleanups) - 1; index >= 0; index-- {
			transport.cleanups[index]()
		}
	})
}

func (transport *transport) addCleanup(cleanup func()) {
	if cleanup != nil {
		transport.cleanups = append(transport.cleanups, cleanup)
	}
}
