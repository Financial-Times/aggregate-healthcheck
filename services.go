package main

import (
	"github.com/coreos/go-etcd/etcd"
	"strings"
)

type ServiceRegistry interface {
	Services() []Service
}

type Service struct {
	Name string
	Host string
}

type CocoServiceRegistry struct {
	etcd      *etcd.Client
	keyPrefix string
}

func (r *CocoServiceRegistry) Services() []Service {
	resp, err := r.etcd.Get(r.keyPrefix, true, false)
	if err != nil {
		panic(err.Error())
	}

	services := make([]Service, len(resp.Node.Nodes))
	for i, node := range resp.Node.Nodes {
		services[i] = Service{Name: strings.TrimPrefix(node.Key, r.keyPrefix), Host: "localhost:8080"}
	}

	return services
}

func NewCocoServiceRegistry(etcd *etcd.Client, keyPrefix string) *CocoServiceRegistry {
	return &CocoServiceRegistry{etcd: etcd, keyPrefix: keyPrefix}
}
