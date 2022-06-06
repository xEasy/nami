package xclient

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type RegistryDiscovery struct {
	*MultiServersDiscovery
	registry      string
	timeout       time.Duration
	lastUpdatedAt time.Time
}

func (r *RegistryDiscovery) Refresh() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lastUpdatedAt.Add(r.timeout).After(time.Now()) {
		return nil
	}
	fmt.Println("rpc server: refresh servers from registry ", r.registry)
	resp, err := http.Get(r.registry)
	if err != nil {
		fmt.Println("rpc server: refresh servers fail ", err)
		return err
	}
	servers := strings.Split(resp.Header.Get("X-Namirpc-Servers"), ",")
	r.servers = make([]string, 0, len(servers))
	for _, server := range servers {
		if strings.TrimSpace(server) != "" {
			r.servers = append(r.servers, strings.TrimSpace(server))
		}
	}
	r.lastUpdatedAt = time.Now()
	return nil
}

func (r *RegistryDiscovery) Update(servers []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.servers = servers
	r.lastUpdatedAt = time.Now()
	return nil
}

func (r *RegistryDiscovery) Get(mode SelectMode) (string, error) {
	if err := r.Refresh(); err != nil {
		return "", err
	}
	return r.MultiServersDiscovery.Get(mode)
}

func (r *RegistryDiscovery) GetAll() ([]string, error) {
	if err := r.Refresh(); err != nil {
		return nil, err
	}
	return r.MultiServersDiscovery.GetAll()
}

const defaultUpdateTimeout = time.Second * 10

func NewRegistryDiscovery(rAddr string, timeout time.Duration) *RegistryDiscovery {
	if timeout == 0 {
		timeout = defaultUpdateTimeout
	}

	return &RegistryDiscovery{
		MultiServersDiscovery: NewMultiServersDiscovery(make([]string, 0)),
		registry:              rAddr,
		timeout:               timeout,
	}
}
