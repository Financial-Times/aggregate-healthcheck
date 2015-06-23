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
	etcd        *etcd.Client
	keyPrefix   string
	vulcandAddr string
	excludes	map[string]bool
}

func (r *CocoServiceRegistry) Services() []Service {
	resp, err := r.etcd.Get(r.keyPrefix, true, false)
	if err != nil {
		panic(err.Error())
	}

	services := make([]Service, len(resp.Node.Nodes))
	for i, node := range resp.Node.Nodes {
		name := strings.TrimPrefix(node.Key, r.keyPrefix)
		if r.excludes[name] != true {
			services[i] = Service{Name: name, Host: r.vulcandAddr}
		}
	}

	return services
}

func NewCocoServiceRegistry(etcd *etcd.Client, keyPrefix, vulcandAddr string, excludeList []string) *CocoServiceRegistry {
	excludes := make(map[string]bool)
	for _, v := range excludeList {
	    excludes[v] = true
	}
	return &CocoServiceRegistry{etcd: etcd, keyPrefix: keyPrefix, vulcandAddr: vulcandAddr, excludes: excludes}
}
