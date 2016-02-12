package main

import (
	"flag"
	"github.com/coreos/etcd/client"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"golang.org/x/net/proxy"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	var (
		socksProxy   = flag.String("socks-proxy", "", "Use specified SOCKS proxy (e.g. localhost:2323)")
		etcdPeers    = flag.String("etcd-peers", "http://localhost:4001", "Comma-separated list of addresses of etcd endpoints to connect to")
		vulcandAddr  = flag.String("vulcand", "localhost:8080", "Vulcand address")
		//graphiteHost = flag.String("graphite-host", "graphite.ft.com", "Graphite host address")
		//graphitePort = flag.Int("graphite-port", 2003, "Graphite port")
		//environment  = flag.String("environment", "local", "Environment tag")
	)
	flag.Parse()

	transport := &http.Transport{Dial: proxy.Direct.Dial}
	if *socksProxy != "" {
		dialer, _ := proxy.SOCKS5("tcp", *socksProxy, nil, proxy.Direct)
		transport.Dial = dialer.Dial
	}

	cfg := client.Config{
		Endpoints:               strings.Split(*etcdPeers, ","),
		Transport:               transport,
		HeaderTimeoutPerRequest: 10 * time.Second,
	}

	etcd, err := client.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	etcdKeysApi := client.NewKeysAPI(etcd)

	checker := NewHttpHealthChecker(&http.Client{Transport: transport, Timeout: 10 * time.Second})
	//graphiteFeeder := NewGraphiteFeeder(*graphiteHost, *graphitePort, *environment)

	registry := NewCocoServiceRegistry(etcdKeysApi, *vulcandAddr, checker)
	registry.redefineCategoryList()
	registry.redefineServiceList()
	registry.updateMeasuredServiceList()

	log.Printf("DEBUG - Nr of registered services: [%v]. Nr of registered categories: [%v]", len(registry.services), len(registry.categories) )

	go registry.watchServices()
	go registry.watchCategories()

	handler := NewController(registry).handle

	r := mux.NewRouter()
	r.HandleFunc("/", handler)
	r.HandleFunc("/__health", handler)
	err = http.ListenAndServe(":8080", handlers.LoggingHandler(os.Stdout, r))
	if err != nil {
		panic(err)
	}
}
