package engines

import (
	"context"
	"net"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

func socksProxyDialer(proxyURL *url.URL) (func(context.Context, string, string) (net.Conn, error), error) {
	dialer, err := proxy.FromURL(proxyURL, &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second})
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
			return contextDialer.DialContext(ctx, network, address)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return dialer.Dial(network, address)
	}, nil
}
