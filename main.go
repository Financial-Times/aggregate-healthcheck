package main

import (
	"flag"
	"github.com/coreos/go-etcd/etcd"
	"github.com/gorilla/mux"
	"golang.org/x/net/proxy"
	"net/http"
	"strings"
)

func main() {
	var (
		socksProxy = flag.String("socks-proxy", "", "Use specified SOCKS proxy (e.g. localhost:2323)")
		etcdPeers  = flag.String("etcd-peers", "http://localhost:4001", "Comma-separated list of addresses of etcd endpoints to connect to")
		keyPrefix  = flag.String("key-prefix", "/vulcand/frontends/", "Key prefix to list of services in etcd")
	)
	flag.Parse()

	var dialer proxy.Dialer = proxy.Direct
	if *socksProxy != "" {
		dialer, _ = proxy.SOCKS5("tcp", *socksProxy, nil, proxy.Direct)
	}

	var transport = &http.Transport{Dial: dialer.Dial}

	etcd := etcd.NewClient(strings.Split(*etcdPeers, ","))
	etcd.SetTransport(transport)

	if (*keyPrefix)[len(*keyPrefix)-1] != '/' {
		*keyPrefix = *keyPrefix + "/"
	}

	registry := NewCocoServiceRegistry(etcd, *keyPrefix)

	checker := NewCocoServiceHealthChecker(dialer)
	handler := CocoAggregateHealthHandler(registry, checker)

	r := mux.NewRouter()
	r.HandleFunc("/", handler)
	r.HandleFunc("/__health", handler)

	err := http.ListenAndServe(":8080", r)
	if err != nil {
		panic(err)
	}
}