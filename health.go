package shrt

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"
)

// CheckHealth calls /healthz on the shrt listening on addr (host:port, as
// Config.Addr has it) and returns nil on a 200. It is what `shrt health`
// runs, since the image has no shell or curl for a container health check.
func CheckHealth(ctx context.Context, addr string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL(addr), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health: %s", resp.Status)
	}
	return nil
}

// healthURL is the /healthz URL for a listen address. An unspecified host
// (0.0.0.0, :: or none) means every interface, so it is reached on loopback.
func healthURL(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr + "/healthz"
	}
	if ip, err := netip.ParseAddr(host); host == "" || (err == nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/healthz"
}
