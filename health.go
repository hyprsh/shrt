package shrt

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"time"
)

// CheckHealthFromEnv calls /healthz on the shrt that SHRT_ADDRESS and
// SHRT_PORT (through getenv, with their defaults) name, and so needs nothing
// else of the configuration, such as the token. It is what `shrt health` runs.
func CheckHealthFromEnv(ctx context.Context, getenv func(string) string) error {
	host, port, err := listenFromEnv(getenv)
	if err != nil {
		return err
	}
	return CheckHealth(ctx, host, port)
}

// CheckHealth calls /healthz on the shrt listening on host and port and
// returns nil on a 200. The image has no shell or curl for a container health
// check, so `shrt health` does it.
func CheckHealth(ctx context.Context, host, port string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL(host, port), nil)
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

// healthURL is the /healthz URL for a listen host and port. An unspecified
// host (0.0.0.0, :: or none) means every interface, so it is reached on
// loopback.
func healthURL(host, port string) string {
	if ip, err := netip.ParseAddr(host); host == "" || (err == nil && ip.IsUnspecified()) {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/healthz"
}
