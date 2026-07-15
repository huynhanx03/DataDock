package engines

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/huynhanx03/datadock/internal/core/entity"
)

func enableSSHTransport(result *transport, connection entity.Connection, dial func(context.Context, string, string) (net.Conn, error)) error {
	hostKeyCallback, err := knownhosts.New(connection.SSHTunnel.KnownHostsPath)
	if err != nil {
		return err
	}
	auth, err := sshAuthMethods(connection.SSHTunnel)
	if err != nil {
		return err
	}
	sshAddress := net.JoinHostPort(connection.SSHTunnel.Host, strconv.Itoa(connection.SSHTunnel.Port))
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	connectionToSSH, err := dial(ctx, "tcp", sshAddress)
	if err != nil {
		return err
	}
	_ = connectionToSSH.SetDeadline(time.Now().Add(12 * time.Second))
	clientConnection, channels, requests, err := ssh.NewClientConn(connectionToSSH, sshAddress, &ssh.ClientConfig{User: connection.SSHTunnel.Username, Auth: auth, HostKeyCallback: hostKeyCallback, Timeout: 12 * time.Second})
	if err != nil {
		connectionToSSH.Close()
		return err
	}
	_ = connectionToSSH.SetDeadline(time.Time{})
	result.sshClient = ssh.NewClient(clientConnection, channels, requests)
	result.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return result.sshClient.Dial(network, address)
	}
	return nil
}

func sshAuthMethods(tunnel entity.SSHTunnel) ([]ssh.AuthMethod, error) {
	methods := make([]ssh.AuthMethod, 0, 2)
	if tunnel.Password != "" {
		methods = append(methods, ssh.Password(tunnel.Password))
	}
	if tunnel.PrivateKeyPath != "" {
		privateKey, err := os.ReadFile(tunnel.PrivateKeyPath)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(privateKey)
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if len(methods) == 0 {
		return nil, errors.New("SSH tunnel requires a password or private key")
	}
	return methods, nil
}
