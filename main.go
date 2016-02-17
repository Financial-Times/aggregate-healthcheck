package main

import (
	"flag"
	"github.com/coreos/etcd/client"
	"github.com/gorilla/mux"
	"golang.org/x/net/proxy"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const logPattern = log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile | log.LUTC

var infoLogger *log.Logger
var warnLogger *log.Logger
var errorLogger *log.Logger

func main() {
	initLogs(os.Stdout, os.Stdout, os.Stderr)
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

func initLogs(infoHandle io.Writer, warnHandle io.Writer, errorHandle io.Writer) {
	//to be used for INFO-level logging: info.Println("foo is now bar")
	infoLogger = log.New(infoHandle, "INFO  - ", logPattern)
	//to be used for WARN-level logging: warn.Println("foo is now bar")
	warnLogger = log.New(warnHandle, "WARN  - ", logPattern)
	//to be used for ERROR-level logging: errorL.Println("foo is now bar")
	errorLogger = log.New(errorHandle, "ERROR - ", logPattern)
}
