package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
)

type addressResolver interface {
	Lookup(context.Context, string) ([]netip.Addr, error)
}

type publicIPDiscoverer interface {
	Discover(context.Context) ([]netip.Addr, error)
}

type dnsResolver struct {
	Resolver *net.Resolver
}

func (resolver dnsResolver) Lookup(ctx context.Context, domain string) ([]netip.Addr, error) {
	backend := resolver.Resolver
	if backend == nil {
		backend = net.DefaultResolver
	}
	addresses, err := backend.LookupNetIP(ctx, "ip", domain)
	if err != nil {
		return nil, fmt.Errorf("resolve %s: %w", domain, err)
	}
	return uniqueAddresses(addresses), nil
}

type httpPublicIPDiscoverer struct {
	IPv4URL string
	IPv6URL string
	Timeout time.Duration
}

func (discoverer httpPublicIPDiscoverer) Discover(ctx context.Context) ([]netip.Addr, error) {
	type result struct {
		address netip.Addr
		err     error
	}
	results := make(chan result, 2)
	var wait sync.WaitGroup
	for _, target := range []struct {
		network string
		url     string
	}{
		{network: "tcp4", url: discoverer.IPv4URL},
		{network: "tcp6", url: discoverer.IPv6URL},
	} {
		wait.Add(1)
		go func(network, endpoint string) {
			defer wait.Done()
			address, err := fetchPublicIP(ctx, network, endpoint, discoverer.Timeout)
			results <- result{address: address, err: err}
		}(target.network, target.url)
	}
	go func() {
		wait.Wait()
		close(results)
	}()

	var addresses []netip.Addr
	var failures []error
	for result := range results {
		if result.err != nil {
			failures = append(failures, result.err)
			continue
		}
		addresses = append(addresses, result.address)
	}
	addresses = uniqueAddresses(addresses)
	if len(addresses) == 0 {
		return nil, fmt.Errorf("determine server public IP: %w", errors.Join(failures...))
	}
	return addresses, nil
}

func fetchPublicIP(ctx context.Context, network, endpoint string, timeout time.Duration) (netip.Addr, error) {
	if endpoint == "" {
		return netip.Addr{}, errors.New("public IP endpoint is empty")
	}
	dialer := &net.Dialer{Timeout: timeout}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	transport.DialContext = func(ctx context.Context, _, address string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, address)
	}
	client := &http.Client{Transport: transport, Timeout: timeout}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return netip.Addr{}, err
	}
	request.Header.Set("User-Agent", "netprobe-deploy")
	response, err := client.Do(request)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("query %s: %w", endpoint, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4<<10))
		return netip.Addr{}, fmt.Errorf("query %s: HTTP %s", endpoint, response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 128))
	if err != nil {
		return netip.Addr{}, fmt.Errorf("read %s: %w", endpoint, err)
	}
	address, err := netip.ParseAddr(strings.TrimSpace(string(body)))
	if err != nil {
		return netip.Addr{}, fmt.Errorf("query %s returned an invalid IP address", endpoint)
	}
	address = address.Unmap()
	if network == "tcp4" && !address.Is4() || network == "tcp6" && !address.Is6() {
		return netip.Addr{}, fmt.Errorf("query %s returned the wrong address family", endpoint)
	}
	return address, nil
}

func verifyDNS(domain string, serverAddresses, domainAddresses []netip.Addr) error {
	serverAddresses = uniqueAddresses(serverAddresses)
	domainAddresses = uniqueAddresses(domainAddresses)
	if len(serverAddresses) == 0 {
		return errors.New("could not determine this server's public IP")
	}
	if len(domainAddresses) == 0 {
		return fmt.Errorf("%s has no A or AAAA records", domain)
	}
	serverSet := make(map[netip.Addr]struct{}, len(serverAddresses))
	for _, address := range serverAddresses {
		serverSet[address.Unmap()] = struct{}{}
	}
	for _, address := range domainAddresses {
		if _, ok := serverSet[address.Unmap()]; ok {
			return nil
		}
	}
	return fmt.Errorf(
		"DNS mismatch: %s resolves to %s, while this server reports %s",
		domain,
		formatAddresses(domainAddresses),
		formatAddresses(serverAddresses),
	)
}

func uniqueAddresses(addresses []netip.Addr) []netip.Addr {
	seen := make(map[netip.Addr]struct{}, len(addresses))
	result := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		if !address.IsValid() {
			continue
		}
		address = address.Unmap()
		if _, ok := seen[address]; ok {
			continue
		}
		seen[address] = struct{}{}
		result = append(result, address)
	}
	sort.Slice(result, func(left, right int) bool {
		return result[left].Compare(result[right]) < 0
	})
	return result
}

func formatAddresses(addresses []netip.Addr) string {
	addresses = uniqueAddresses(addresses)
	values := make([]string, 0, len(addresses))
	for _, address := range addresses {
		values = append(values, address.String())
	}
	return strings.Join(values, ", ")
}
