package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	etcdClient "github.com/coreos/etcd/client"
	"github.com/gorilla/mux"
	"github.com/jawher/mow.cli"
	"golang.org/x/net/proxy"
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
	severityOneApps := app.String(cli.StringOpt{
		Name:   "sev-1-apps",
		Value:  "synthetic-list-publication-monitor,synthetic-article-publication-monitor,synthetic-image-publication-monitor,publish-availability-monitor",
		Desc:   "Comma-separated list of sev 1 apps",
		EnvVar: "SEV_1_APPS",
	})

	app.Action = func() {
		initLogs(os.Stdout, os.Stdout, os.Stderr)
		transport := &http.Transport{
			Dial: proxy.Direct.Dial,
			ResponseHeaderTimeout: 10 * time.Second,
			MaxIdleConnsPerHost:   100,
		}
		if *socksProxy != "" {
			dialer, _ := proxy.SOCKS5("tcp", *socksProxy, nil, proxy.Direct)
			transport.Dial = dialer.Dial
		}
		httpClient := &http.Client{
			Timeout:   5 * time.Second,
			Transport: transport,
		}

		sos := strings.Split(*severityOneApps, ",")
		checker := NewHTTPHealthChecker(httpClient, sos)

		cfg := etcdClient.Config{
			Endpoints:               strings.Split(*etcdPeers, ","),
			Transport:               transport,
			HeaderTimeoutPerRequest: 10 * time.Second,
		}
		etcd, err := etcdClient.New(cfg)
		if err != nil {
			log.Fatal(err)
		}
		etcdKeysAPI := etcdClient.NewKeysAPI(etcd)

		registry := NewCocoServiceRegistry(etcdKeysAPI, *vulcandAddr, checker, *environment)
		registry.redefineCategoryList()
		registry.redefineServiceList()
		registry.redefineClusterAck()
		registry.updateMeasuredServiceList()

		go registry.watchServices()
		go registry.watchCategories()
		go registry.watchClusterAck()

		graphiteFeeder := NewGraphiteFeeder(*graphiteHost, *graphitePort, *environment, registry)
		go graphiteFeeder.feed()

		controller := NewController(registry, environment)

		handler := controller.handleHealthcheck
		gtgHandler := controller.handleGoodToGo
		aggHandler := controller.handleAggHealthcheck
		r := mux.NewRouter()
		r.HandleFunc("/", handler)
		r.HandleFunc("/__health", handler)
		r.HandleFunc("/__gtg", gtgHandler)
		r.HandleFunc("/__agghealth", aggHandler)
		err = http.ListenAndServe(":8080", r)
		if err != nil {
			errorLogger.Println("Can't set up HTTP listener on 8080.")
			os.Exit(0)
		}
	}
	app.Run(os.Args)
}

func initLogs(infoHandle io.Writer, warnHandle io.Writer, errorHandle io.Writer) {
	infoLogger = log.New(infoHandle, "INFO  - ", logPattern)
	warnLogger = log.New(warnHandle, "WARN  - ", logPattern)
	errorLogger = log.New(errorHandle, "ERROR - ", logPattern)
}
