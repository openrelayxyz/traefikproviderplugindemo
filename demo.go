// Package pluginproviderdemo contains a demo of the provider's plugin.
package pluginproviderdemo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
	// "net/http"
	"strconv"

	"github.com/traefik/genconf/dynamic"
	// "github.com/traefik/genconf/dynamic/tls"
	// "github.com/gorilla/websocket"
	"golang.org/x/net/websocket"
	"strings"
)

// Config the plugin configuration.
type Config struct {
	Providers []ServiceProvider `json:"providers`
	Methods []RPCMethod `json:"methods"`
}

type ServiceProvider struct {
	URL string `json:"url"`
	WSURL string `json:"wsurl"`
	Methods []string `json:"methods"`
	Archive bool `json:"archive"`
	latestBlock int64
}

type newHeadsMessage struct {
	Params struct {
		Result struct {
			Number string
		} `json:"result"`
	} `json:"params"`
}

func (sp *ServiceProvider) MonitorBlock(ctx context.Context, ch chan<- int64) {
	if sp.WSURL == "" {
		return
	}
	var conn *websocket.Conn
	var err error
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	go func () {
		for {
			if ctx.Err() != nil {
				return
			}
			websocket.Dial(sp.WSURL, "", sp.URL)
			// conn, _, err = websocket.DefaultDialer.Dial(sp.WSURL, nil)
			if err != nil {
				log.Printf("Failed to dial websockets for %v: %v", sp.WSURL, err.Error())
			}
			conn.Write([]byte(`{"id":0, "jsonrpc":"2.0", "method":"eth_subscribe", "params": ["newHeads"]}`))
			msg := make([]byte, 2048)
			_, err = conn.Read(msg) 
			for {
				_, err = conn.Read(msg) 
				if err != nil {
					log.Printf("Failed to read message on websockets for %v: %v", sp.WSURL, err.Error())
					break
				}
				var nhm newHeadsMessage
				json.Unmarshal(msg, &nhm)
				num, err := strconv.ParseInt(nhm.Params.Result.Number, 0, 64)
				if err != nil {
					log.Printf("Error parsing number from head: %v - %v", nhm.Params.Result.Number, err.Error())
				} else {
					sp.latestBlock = num
					ch <- num
				}

			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
}

type RPCMethod struct {
	Name string `json:"name"`
	BlockSensitive bool `json:"blockSensitive"`
	BlockSpecific bool `json:"blockSpecific"`
	BlockArg bool `json:"blockArg"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		Methods: []RPCMethod{},
		Providers: []ServiceProvider{},
	}
}

// Provider a simple provider plugin.
type Provider struct {
	name    string
	methods []RPCMethod
	serviceProvider map[string][]*ServiceProvider
	blockNumCh <-chan int64

	cancel  func()
}

// New creates a new Provider plugin.
func New(ctx context.Context, config *Config, name string) (*Provider, error) {
	sp := make(map[string][]*ServiceProvider)
	ch := make(chan int64)
	for _, provider := range config.Providers {
		for _, m := range provider.Methods {
			if _, ok := sp[m]; !ok {
				sp[m] = []*ServiceProvider{}
			}
			sp[m] = append(sp[m], &provider)
		}
		provider.MonitorBlock(ctx, ch)
	}
	return &Provider{
		name:         name,
		methods: config.Methods,
		serviceProvider: sp,
		blockNumCh: ch,
	}, nil
}

// Init the provider.
func (p *Provider) Init() error {
	return nil
}

// Provide creates and send dynamic configuration.
func (p *Provider) Provide(cfgChan chan<- json.Marshaler) error {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel

	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Print(err)
			}
		}()

		p.loadConfiguration(ctx, cfgChan)
	}()

	return nil
}

func (p *Provider) loadConfiguration(ctx context.Context, cfgChan chan<- json.Marshaler) {
	var bn int64
	for {
		select {
		case block := <-p.blockNumCh:
			if block > bn {
				bn = block
				configuration := p.generateConfiguration(bn)
				cfgChan <- &dynamic.JSONPayload{Configuration: configuration}
			}

		case <-ctx.Done():
			return
		}
	}
}

// Stop to stop the provider and the related go routines.
func (p *Provider) Stop() error {
	p.cancel()
	return nil
}

func(p *Provider) generateConfiguration(bn int64) *dynamic.Configuration {
	configuration := &dynamic.Configuration{
		HTTP: &dynamic.HTTPConfiguration{
			Routers:           make(map[string]*dynamic.Router),
			Middlewares:       make(map[string]*dynamic.Middleware),
			Services:          make(map[string]*dynamic.Service),
			ServersTransports: make(map[string]*dynamic.ServersTransport),
		},
	}

	configuration.HTTP.Routers["loopback"] = &dynamic.Router{
		EntryPoints: []string{"web"},
		Service:     "loopback",
		Rule:        "Host(`/router`)",
	}

	configuration.HTTP.Middlewares["rpcloopback"] = &dynamic.Middleware{
		Plugin: make(map[string]dynamic.PluginConf),
	}
	configuration.HTTP.Middlewares["rpcloopback"].Plugin["rpcloopback"] = dynamic.PluginConf{}

	configuration.HTTP.Services["loopback"] = &dynamic.Service{
		LoadBalancer: &dynamic.ServersLoadBalancer{
			Servers: []dynamic.Server{
				{
					URL: "http://localhost:8000",
				},
			},
			PassHostHeader: boolPtr(true),
		},
	}

	for _, method := range p.methods {
		servers := []dynamic.Server{}
		for _, sp := range p.serviceProvider[method.Name] {
			if sp.latestBlock >= bn {
				servers = append(servers, dynamic.Server{
					URL: sp.URL,
				})
			}
		}
		configuration.HTTP.Services[method.Name] = &dynamic.Service{
			LoadBalancer: &dynamic.ServersLoadBalancer{
				PassHostHeader: boolPtr(false),
				Servers: servers,
			},
		}
		path := "/" + strings.Join(strings.Split(method.Name, "_"), "/")
		configuration.HTTP.Routers[method.Name] = &dynamic.Router{
			EntryPoints: []string{"web"},
			Service:     method.Name,
			Rule:        fmt.Sprintf("PathPrefix(`%v`)", path),
		}
	}

	return configuration
}

func boolPtr(v bool) *bool {
	return &v
}
