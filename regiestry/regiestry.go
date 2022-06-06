package regiestry

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// ServerItem represent as rpc server obj
type ServerItem struct {
	Addr  string
	start time.Time
}

type Regiestry struct {
	timeout time.Duration
	mu      sync.Mutex
	servers map[string]*ServerItem
}

const (
	defautPath     = "/_namirpc_/regiest"
	defaultTimeout = time.Second * 300
)

func New(time time.Duration) *Regiestry {
	return &Regiestry{
		timeout: time,
		servers: make(map[string]*ServerItem),
	}
}

var DefaultRegiestry = New(defaultTimeout)

func (r *Regiestry) putServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.servers[addr]
	if s == nil {
		r.servers[addr] = &ServerItem{Addr: addr, start: time.Now()}
	} else {
		s.start = time.Now()
	}
}

func (r *Regiestry) aliveServers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var alive []string
	for addr, s := range r.servers {
		if r.timeout == 0 || s.start.Add(r.timeout).After(time.Now()) {
			alive = append(alive, addr)
		} else {
			delete(r.servers, addr)
		}
	}
	sort.Strings(alive)
	return alive
}

func (r *Regiestry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		w.Header().Set("X-Namirpc-Servers", strings.Join(r.aliveServers(), ","))
	case "POST":
		addr := w.Header().Get("X-Namirpc-Server")
		if addr == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.putServer(addr)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *Regiestry) HandlHTTP(registryPath string) {
	http.Handle(registryPath, r)
	fmt.Println("rpc server: regiest path ", registryPath)
}

func HandlHTTP() {
	DefaultRegiestry.HandlHTTP(defautPath)
}
