package main

import (
	"flag"
	"github.com/coreos/etcd/client"
	"github.com/gorilla/mux"
	"golang.org/x/net/proxy"
	"log"
	"net/http"
	"strings"
	"time"
)

func main() {
	var (
		socksProxy   = flag.String("socks-proxy", "", "Use specified SOCKS proxy (e.g. localhost:2323)")
		etcdPeers    = flag.String("etcd-peers", "http://localhost:4001", "Comma-separated list of addresses of etcd endpoints to connect to")
		vulcandAddr  = flag.String("vulcand", "localhost:8080", "Vulcand address")
		graphiteHost = flag.String("graphite-host", "graphite.ft.com", "Graphite host address")
		graphitePort = flag.Int("graphite-port", 2003, "Graphite port")
		environment  = flag.String("environment", "local", "Environment tag")
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

	checker := NewHttpHealthChecker(&http.Client{Transport: transport, Timeout: 5 * time.Second})

	registry := NewCocoServiceRegistry(etcdKeysApi, *vulcandAddr, checker)
	registry.redefineCategoryList()
	registry.redefineServiceList()
	registry.updateMeasuredServiceList()

	go registry.watchServices()
	go registry.watchCategories()

	graphiteFeeder := NewGraphiteFeeder(*graphiteHost, *graphitePort, *environment, registry)
	go graphiteFeeder.feed()

	controller := NewController(registry, environment)

	handler := controller.handleHealthcheck
	gtgHandler := controller.handleGoodToGo
	r := mux.NewRouter()
	r.HandleFunc("/", handler)
	r.HandleFunc("/__health", handler)
	r.HandleFunc("/__gtg", gtgHandler)
	err = http.ListenAndServe(":8080", r)
	if err != nil {
		panic(err)
	}
}
