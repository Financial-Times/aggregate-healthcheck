package main

import (
	"fmt"
	"github.com/coreos/go-etcd/etcd"
	"log"
	"strings"
)

type ServiceRegistry interface {
	Services() []Service
}

type Service struct {
	Name        string
	Host        string
	Healthcheck string
}

type CocoServiceRegistry struct {
	etcd        *etcd.Client
	keyPrefix   string
	vulcandAddr string
}

func (r *CocoServiceRegistry) Services() []Service {
	resp, err := r.etcd.Get(r.keyPrefix, true, true)
	if err != nil {
		panic(err.Error())
	}

	services := []Service{}
	for _, service := range resp.Node.Nodes {
		log.Printf("INFO Service: %s", service)
		resp, err := r.etcd.Get(service.Key, false, false)
		if err == nil {
			name := strings.TrimPrefix(service.Key, r.keyPrefix)
			healthcheck := fmt.Sprintf("/health/%s%s", name, resp.Node.Value)
			log.Printf("INFO Healtheck: %s", healthcheck)
			services = append(services, Service{Name: name, Host: r.vulcandAddr, Healthcheck: healthcheck})
		}
	}

	return services
}

func NewCocoServiceRegistry(etcd *etcd.Client, keyPrefix, vulcandAddr string) *CocoServiceRegistry {
	return &CocoServiceRegistry{etcd: etcd, keyPrefix: keyPrefix, vulcandAddr: vulcandAddr}
}
