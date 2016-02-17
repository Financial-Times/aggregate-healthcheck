package main

import (
	"github.com/coreos/etcd/client"
	"github.com/gorilla/mux"
	"golang.org/x/net/proxy"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"github.com/jawher/mow.cli"
)

const logPattern = log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile | log.LUTC

var infoLogger *log.Logger
var warnLogger *log.Logger
var errorLogger *log.Logger

func main() {
	app := cli.App("aggregate-healthcheck", "Monitoring health of multiple services in cluster.")
	socksProxy := app.String(cli.StringOpt{
		Name:   "socks-proxy",
		Value:  "",
		Desc:   "Use specified SOCKS proxy (e.g. localhost:2323)",
		EnvVar: "SOCKS_PROXY",
	})
	etcdPeers := app.String(cli.StringOpt{
		Name:   "etcd-peers",
		Value:  "http://localhost:4001",
		Desc:   "Comma-separated list of addresses of etcd endpoints to connect to",
		EnvVar: "ETCD_PEERS",
	})
	vulcandAddr := app.String(cli.StringOpt{
		Name:   "vulcand",
		Value:  "localhost:8080",
		Desc:   "Vulcand address",
		EnvVar: "VULCAND_ADDRESS",
	})
	graphiteHost := app.String(cli.StringOpt{
		Name:   "graphite-host",
		Value:  "graphite.ft.com",
		Desc:   "Graphite host address",
		EnvVar: "GRAPHITE_HOST",
	})
	graphitePort := app.Int(cli.IntOpt{
		Name:   "graphite-port",
		Value:  2003,
		Desc:   "Graphite port",
		EnvVar: "GRAPHITE_PORT",
	})
	environment := app.String(cli.StringOpt{
		Name:   "environment",
		Value:  "local",
		Desc:   "Environment tag (e.g. local, pre-prod, prod-uk)",
		EnvVar: "ENVIRONMENT",
	})

	app.Action = func() {
		initLogs(os.Stdout, os.Stdout, os.Stderr)
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
			errorLogger.Println("Can't set up HTTP listener on 8080.")
			os.Exit(0)
		}
	}
	app.Run(os.Args)
}

func initLogs(infoHandle io.Writer, warnHandle io.Writer, errorHandle io.Writer) {
	//to be used for INFO-level logging: info.Println("foo is now bar")
	infoLogger = log.New(infoHandle, "INFO  - ", logPattern)
	//to be used for WARN-level logging: warn.Println("foo is now bar")
	warnLogger = log.New(warnHandle, "WARN  - ", logPattern)
	//to be used for ERROR-level logging: errorL.Println("foo is now bar")
	errorLogger = log.New(errorHandle, "ERROR - ", logPattern)
}
