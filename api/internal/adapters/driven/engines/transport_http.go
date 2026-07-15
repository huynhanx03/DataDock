package engines

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"
)

func proxyDialer(rawURL, username, password string) (func(context.Context, string, string) (net.Conn, error), error) {
	proxyURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if username != "" {
		proxyURL.User = url.UserPassword(username, password)
	}
	proxyAddress := proxyURL.Host
	if _, _, err := net.SplitHostPort(proxyAddress); err != nil {
		defaultPort := "80"
		if proxyURL.Scheme == "https" {
			defaultPort = "443"
		}
		proxyAddress = net.JoinHostPort(proxyURL.Hostname(), defaultPort)
	}
	if proxyURL.Scheme == "socks5" || proxyURL.Scheme == "socks5h" {
		return socksProxyDialer(proxyURL)
	}
	if proxyURL.Scheme != "http" && proxyURL.Scheme != "https" {
		return nil, errors.New("unsupported proxy protocol")
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		connection, err := dialer.DialContext(ctx, "tcp", proxyAddress)
		if err != nil {
			return nil, err
		}
		if proxyURL.Scheme == "https" {
			secureConnection := tls.Client(connection, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: proxyURL.Hostname()})
			if err := secureConnection.HandshakeContext(ctx); err != nil {
				connection.Close()
				return nil, err
			}
			connection = secureConnection
		}
		request := &http.Request{Method: http.MethodConnect, URL: &url.URL{Opaque: address}, Host: address, Header: make(http.Header)}
		if proxyURL.User != nil {
			proxyPassword, _ := proxyURL.User.Password()
			request.SetBasicAuth(proxyURL.User.Username(), proxyPassword)
		}
		if err := request.Write(connection); err != nil {
			connection.Close()
			return nil, err
		}
		reader := bufio.NewReader(connection)
		response, err := http.ReadResponse(reader, request)
		if err != nil {
			connection.Close()
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			connection.Close()
			return nil, fmt.Errorf("proxy CONNECT failed: %s", response.Status)
		}
		response.Body.Close()
		return &bufferedConn{Conn: connection, reader: reader}, nil
	}, nil
}
