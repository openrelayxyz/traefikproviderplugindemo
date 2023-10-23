// Package pluginproviderdemo contains a demo of the provider's plugin.
package pluginproviderdemo

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
	"strconv"
	"net/http"
	"io/ioutil"

	"github.com/traefik/genconf/dynamic"

	"strings"
)

// Config the plugin configuration.
type Config struct {
	Providers []*ServiceProvider `json:"providers`
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
			Number string `json:"number"`
		} `json:"result"`
	} `json:"params"`
}

// func (sp *ServiceProvider)  MonitorBlock(ctx context.Context, out chan int64)  {

// 	// // Start the routine to connect to the websocket and fetch the numbers
// 	go func() {

// 		// Connect to the WebSocket
// 		conn, _, _, err := ws.DefaultDialer.Dial(nil, url)
// 		if err != nil {
// 			log.Fatalf("Failed to connect to the websocket: %v", err)
// 			return
// 		}
// 		defer conn.Close()

// 		// Send the subscription message
// 		msg := []byte(`{"id":0, "jsonrpc":"2.0", "method":"eth_subscribe", "params": ["newHeads"]}`)
// 		if err := wsutil.WriteClientText(conn, msg); err != nil {
// 			log.Fatalf("Failed to send the subscription message: %v", err)
// 			return
// 		}

// 		// Loop and read messages from the websocket
// 		for {
// 			// Read the next message
// 			header, err := ws.ReadHeader(conn)
// 			if err != nil {
// 				log.Printf("Failed to read message header: %v", err)
// 				return
// 			}

// 			payload := make([]byte, header.Length)
// 			_, err = io.ReadFull(conn, payload)
// 			if err != nil {
// 				log.Printf("Failed to read message payload: %v", err)
// 				return
// 			}

// 			// Decrypt the message if it's masked
// 			if header.Masked {
// 				ws.Cipher(payload, header.Mask, 0)
// 			}

// 			var nhm newHeadsMessage
// 			if err := json.Unmarshal(payload, &nhm); err != nil {
// 				log.Printf("Failed to parse message: %v", err)
// 				continue
// 			}

// 			// Send the block number over the channel
// 				num, err := strconv.ParseInt(nhm.Params.Result.Number, 0, 64)
// 				if err != nil {
// 					log.Printf("Error parsing number from head: %v - %v", nhm.Params.Result.Number, err.Error())
// 				} else {
// 					sp.latestBlock = num
// 					ch <- num
// 				}

// 		}
// 	}()

// }


// func (sp *ServiceProvider) MonitorBlock(ctx context.Context, ch chan<- int64) {
// 	if sp.WSURL == "" {
// 		return
// 	}
// 	var conn *websocket.Conn
// 	var err error
// 	go func() {
// 		<-ctx.Done()
// 		conn.Close()
// 	}()
// 	go func () {
// 		for {
// 			if ctx.Err() != nil {
// 				return
// 			}
// 		// 	// websocket.Dial(sp.WSURL, "", sp.URL)
// 			conn, _, err = websocket.DefaultDialer.Dial(sp.WSURL, nil)
// 			if err != nil {
// 				log.Printf("Failed to dial websockets for %v: %v", sp.WSURL, err.Error())
// 				time.Sleep(100 * time.Millisecond)
// 				continue
// 			}
// 			conn.WriteMessage(websocket.TextMessage, []byte(`{"id":0, "jsonrpc":"2.0", "method":"eth_subscribe", "params": ["newHeads"]}`))
// 			_, msg, err := conn.ReadMessage() 
// 			log.Printf("Message (%v): %v", err, msg)
// 			// for {
// 			// 	_, msg, err = conn.ReadMessage() 
// 			// 	if err != nil {
// 			// 		log.Printf("Failed to read message on websockets for %v: %v", sp.WSURL, err.Error())
// 			// 		break
// 			// 	}
// 		// 		var nhm newHeadsMessage
// 		// 		json.Unmarshal(msg, &nhm)
// 		// 		num, err := strconv.ParseInt(nhm.Params.Result.Number, 0, 64)
// 		// 		if err != nil {
// 		// 			log.Printf("Error parsing number from head: %v - %v", nhm.Params.Result.Number, err.Error())
// 		// 		} else {
// 		// 			sp.latestBlock = num
// 		// 			ch <- num
// 		// 		}

// 			// }
// 			time.Sleep(100 * time.Millisecond)
// 		}
// 	}()
// }

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
		Providers: []*ServiceProvider{},
	}
}

// Provider a simple provider plugin.
type Provider struct {
	name    string
	methods []RPCMethod
	serviceProvider map[string][]*ServiceProvider
	blockNumCh chan int64

	cancel  func()
}


func Serve(ctx context.Context, providers []*ServiceProvider, port int64, blockNumCh chan int64) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			data, _ := json.Marshal(providers)
			w.Write(data)
			return
		}
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading body: %v", err)
			http.Error(w, "Server Error1", http.StatusInternalServerError)
			return
		}
		var rpc map[string]int64
		if err := json.Unmarshal(body, &rpc); err != nil {
			log.Printf("Error unmarshalling")
			return
		}
		var max int64
		for k, v := range rpc {
			idx, err := strconv.Atoi(k)
			if err != nil {
				http.Error(w, "Bad body", http.StatusInternalServerError)
				return
			}
			providers[idx].latestBlock = v
			if v > max {
				max = v
			}
		}
		blockNumCh <- max
		w.Write([]byte(`{"ok": true}`))
	})
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", port), nil))
	
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
			sp[m] = append(sp[m], provider) // TODO: This seems to be broken because loop variables or something
		}
		// provider.MonitorBlock(ctx, ch)
	}
	go Serve(ctx, config.Providers, 7777, ch)
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
	p.blockNumCh <- 0

	return nil
}

func (p *Provider) loadConfiguration(ctx context.Context, cfgChan chan<- json.Marshaler) {
	var bn int64
	for {
		select {
		case block := <-p.blockNumCh:
			if block >= bn {
				bn = block
				configuration := p.generateConfiguration(bn)
				x, _ := json.Marshal(configuration)
				log.Printf("Config: %v", string(x))
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
		Rule:        "PathPrefix(`/router`)",
		Middlewares: []string{"rpcloopback"},
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
			log.Printf("Method: %v, SP: %v", method.Name, sp.URL)
			if sp.latestBlock >= bn {
				servers = append(servers, dynamic.Server{URL: sp.URL})
			}
		}
		if len(servers) == 0 {
			log.Printf("Warning: method %v has no health providers. Balancing across all.",  method.Name)
			for _, sp := range p.serviceProvider[method.Name] {
				servers = append(servers, dynamic.Server{URL: sp.URL})
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
