package main

import (
	"github.com/coreos/go-etcd/etcd"
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
		resp, err := r.etcd.Get(service.Key, false, false)
		if err == nil {
			name := strings.TrimPrefix(service.Key, r.keyPrefix)
			services = append(services, Service{Name: name, Host: r.vulcandAddr, Healthcheck: resp.Node.Value})
			break
		}
	}

	return services
}

func NewCocoServiceRegistry(etcd *etcd.Client, keyPrefix, vulcandAddr string) *CocoServiceRegistry {
	return &CocoServiceRegistry{etcd: etcd, keyPrefix: keyPrefix, vulcandAddr: vulcandAddr}
}
