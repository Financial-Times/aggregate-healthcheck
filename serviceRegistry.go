package main

import (
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
	"log"
	"time"
	"strconv"
	"path/filepath"
)

const (
	servicesKeyPre = "/ft/healthcheck"
	categoriesKeyPre = "/ft/healthcheck-categories"
	periodKeySuffix = "/period_seconds"
	resilientSuffix = "/is_resilient"
	pathSuffix = "/path"
	categoriesSuffix = "/categories"
	defaultDuration = time.Duration(60 * time.Second)
	defaultPath = "__health"
	defaultCategory = Category{"default", defaultDuration, false}
)

type Service struct {
	Name       string
	Host       string
	Path       string
	Categories *[]Category
}

type Category struct {
	Name        string
	Period      time.Duration
	IsResilient bool
}

type CocoServiceRegistry struct {
	etcd        client.KeysAPI
	vulcandAddr string
	services    *map[string]Service
	categories  *map[string]Category
}

func NewCocoServiceRegistry(kapi client.KeysAPI, keyPrefix, vulcandAddr string) *CocoServiceRegistry {
	return &CocoServiceRegistry{etcd: kapi, keyPrefix: keyPrefix, vulcandAddr: vulcandAddr}
}

func (r *CocoServiceRegistry) maintainServices() {
	servicesWatcher := r.etcd.Watcher(servicesKeyPre, &client.WatcherOptions{0, true})
	for {
		_, err := servicesWatcher.Next(context.Background())
		if err != nil {
			log.Printf("ERROR - Error waiting for change in registered services in etcd. %v\n Sleeping 10s...", err.Error())
			time.Sleep(10 * time.Second)
			continue
		}
		r.redefineServices()
	}
}

func (r *CocoServiceRegistry) maintainCategories() {
	categoriesWatcher := r.etcd.Watcher(categoriesKeyPre, &client.WatcherOptions{0, true})
	for {
		_, err := categoriesWatcher.Next(context.Background())
		if err != nil {
			log.Printf("ERROR - Error waiting for change in registered categories in etcd. %v\n Sleeping 10s...", err.Error())
			time.Sleep(10 * time.Second)
			continue
		}
		r.redefineCategories()
	}
}

func (r * CocoServiceRegistry) redefineServices() {
	services := make(map[string]Service)
	servicesResp, err := r.etcd.Get(context.Background(), &client.GetOptions{Sort: true})
	if err != nil {
		log.Printf("ERROR - Failed to get value from %v: %v.", servicesKeyPre, err.Error())
		return
	}
	if !servicesResp.Node.Dir {
		log.Printf("ERROR - %v is not a directory", servicesResp.Node.Key)
		return
	}
	for _, serviceNode := range servicesResp.Node.Nodes {
		if !serviceNode.Dir {
			log.Printf("ERROR - %v is not a directory", serviceNode.Key)
			return
		}
		name := filepath.Base(servicesResp.Node.Key)
		path := defaultPath
		pathResp, err := r.etcd.Get(context.Background(), serviceNode.Key + pathSuffix, &client.GetOptions{Sort: true})
		if err != nil {
			log.Printf("WARN - Failed to get health check path from %v: %v. Using default %v", serviceNode.Key, err.Error(), defaultPath)
		} else {
			path = pathResp.Node.Value
		}
		categories := [1]Category{defaultCategory}
		categoriesResp, err := r.etcd.Get(context.Background(), serviceNode.Key + categoriesSuffix, &client.GetOptions{Sort: true})
		if err != nil {
			log.Printf("WARN - Failed to get app category from %v: %v. Using default 'default'", serviceNode.Key, err.Error())
		} else {
			for _, categoryNode := range categoriesResp.Node.Nodes {
				categoryName := filepath.Base(categoryNode.Key)
				category := r.categories[categoryName]
				if category != nil {
					categories = append(categories, category)
				}
			}
		}
		services = append(services, Service{Name: name, Host: r.vulcandAddr, Path: path, Categories: &categories})
	}
	r.services = &services
}

func (r * CocoServiceRegistry) redefineCategories() {
	categories := make(map[string]Category)
	categoriesResp, err := r.etcd.Get(context.Background(), categoriesKeyPre, &client.GetOptions{Sort: true})
	if err != nil {
		log.Printf("ERROR - Failed to get value from %v: %v.", categoriesKeyPre, err.Error())
		return
	}
	if !categoriesResp.Node.Dir {
		log.Printf("ERROR - %v is not a directory", categoriesResp.Node.Key)
		return
	}
	for _, categoryNode := range categoriesResp.Node.Nodes {
		if !categoryNode.Dir {
			log.Printf("ERROR - %v is not a directory", categoryNode.Key)
			return
		}
		name := filepath.Base(categoryNode.Key)
		periodResp, err := r.etcd.Get(context.Background(), categoryNode.Key + periodKeySuffix, &client.GetOptions{Sort: true})
		period := defaultDuration
		if err != nil {
			log.Printf("WARN - Failed to get health check period from %v: %v. Using default %v", categoryNode.Key, err.Error(), defaultDuration)
		} else {
			periodInt, err2 := strconv.Atoi(periodResp.Node.Value)
			if err2 != nil {
				log.Printf("WARN - Error reading health check period value '%v'. Using default %v", periodResp.Node.Value, defaultDuration)
				periodInt = defaultDuration
			}
			period = periodInt * time.Second
		}
		resilientResp, err := r.etcd.Get(context.Background(), categoryNode.Key + resilientSuffix, nil)
		resilient := false
		if err != nil {
			log.Printf("WARN - Failed to get resilient setting from %v: %v. Using default: false.\n", categoryNode.Key, err.Error())
		} else {
			resilientBool, err2 := strconv.ParseBool(periodResp.Node.Value)
			if err2 != nil {
				log.Printf("WARN - Error reading resilient setting '%v'. Using default: false.", resilientResp.Node.Value)
				resilientBool = false
			}
			resilient = resilientBool
		}

		categories[name] = Category{Name: name, Period: period, IsResilient: resilient}
	}
	r.categories = &categories
}
