package main

import (
	"flag"
	"github.com/coreos/go-etcd/etcd"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"golang.org/x/net/proxy"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	var (
		socksProxy = flag.String("socks-proxy", "", "Use specified SOCKS proxy (e.g. localhost:2323)")
		etcdPeers  = flag.String("etcd-peers", "http://localhost:4001", "Comma-separated list of addresses of etcd endpoints to connect to")
		keyPrefix  = flag.String("key-prefix", "/services/", "Key prefix to list of services in etcd")
		vulcand    = flag.String("vulcand", "localhost:8080", "Vulcand address")
	)

	flag.Parse()

	transport := &http.Transport{Dial: proxy.Direct.Dial}

	if *socksProxy != "" {
		dialer, _ := proxy.SOCKS5("tcp", *socksProxy, nil, proxy.Direct)
		transport.Dial = dialer.Dial
	}

	if (*keyPrefix)[len(*keyPrefix)-1] != '/' {
		*keyPrefix = *keyPrefix + "/"
	}

	etcd := etcd.NewClient(strings.Split(*etcdPeers, ","))
	etcd.SetTransport(transport)

	registry := NewCocoServiceRegistry(etcd, *keyPrefix, *vulcand)
	checker := NewCocoServiceHealthChecker(&http.Client{Transport: transport, Timeout: 5 * time.Second})
	handler := CocoAggregateHealthHandler(registry, checker)

	r := mux.NewRouter()
	r.HandleFunc("/", handler)
	r.HandleFunc("/__health", handler)

	err := http.ListenAndServe(":8080", handlers.LoggingHandler(os.Stdout, r))
	if err != nil {
		panic(err)
	}
}
