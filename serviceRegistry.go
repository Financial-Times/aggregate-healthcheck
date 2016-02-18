package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	fthealth "github.com/Financial-Times/go-fthealth/v1a"
	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

const (
	servicesKeyPre   = "/ft/healthcheck"
	categoriesKeyPre = "/ft/healthcheck-categories"
	periodKeySuffix  = "/period_seconds"
	resilientSuffix  = "/is_resilient"
	pathSuffix       = "/path"
	categoriesSuffix = "/categories"
	defaultDuration  = time.Duration(60 * time.Second)
	pathPre          = "/health/%s%s"
	defaultPath      = "/__health"
)

type Service struct {
	Name       string
	Host       string
	Path       string
	Categories []string
}

type Category struct {
	Name        string
	Period      time.Duration
	IsResilient bool
}

type MeasuredService struct {
	service         *Service
	cachedHealth    *CachedHealth    //latest healthiness measurement
	bufferedHealths *BufferedHealths //up to 60 healthiness measurements to be buffered and sent at once graphite
}

func NewMeasuredService(service *Service) MeasuredService {
	cachedHealth := NewCachedHealth()
	bufferedHealths := NewBufferedHealths()
	go cachedHealth.maintainLatest()
	return MeasuredService{service, cachedHealth, bufferedHealths}
}

type servicesMap map[string]Service
type categoriesMap map[string]Category

type ServiceRegistry struct {
	etcd             client.KeysAPI
	vulcandAddr      string
	checker          HealthChecker
	services         servicesMap
	categories       categoriesMap
	measuredServices map[string]MeasuredService
}

func NewCocoServiceRegistry(etcd client.KeysAPI, vulcandAddr string, checker HealthChecker) *ServiceRegistry {
	services := make(map[string]Service)
	categories := make(map[string]Category)
	measuredServices := make(map[string]MeasuredService)
	return &ServiceRegistry{etcd, vulcandAddr, checker, services, categories, measuredServices}
}

func (r *ServiceRegistry) watchServices() {
	watcher := r.etcd.Watcher(servicesKeyPre, &client.WatcherOptions{AfterIndex: 0, Recursive: true})
	for {
		_, err := watcher.Next(context.Background())
		if err != nil {
			errorLogger.Printf("Error waiting for change under %v in etcd. %v\n Sleeping 10s...", servicesKeyPre, err.Error())
			time.Sleep(10 * time.Second)
			continue
		}
		r.redefineServiceList()
		r.updateMeasuredServiceList()
	}
}

func (r *ServiceRegistry) updateMeasuredServiceList() {
	// adding new services, not touching existing
	for key := range r.services {
		service := r.services[key]
		if mService, ok := r.measuredServices[service.Name]; !ok || !reflect.DeepEqual(service, r.measuredServices[service.Name].service) {
			if ok {
				mService.cachedHealth.terminate <- true
			}
			newMService := NewMeasuredService(&service)
			r.measuredServices[service.Name] = newMService
			go r.scheduleCheck(&newMService, time.NewTimer(0))
		}
	}

	// removing services that don't exist, not touching the rest
	for _, mService := range r.measuredServices {
		if _, ok := r.services[mService.service.Name]; !ok {
			delete(r.measuredServices, mService.service.Name)
			mService.cachedHealth.terminate <- true
		}
	}
}

func (r *ServiceRegistry) watchCategories() {
	watcher := r.etcd.Watcher(categoriesKeyPre, &client.WatcherOptions{AfterIndex: 0, Recursive: true})
	for {
		_, err := watcher.Next(context.Background())
		if err != nil {
			errorLogger.Printf("Error waiting for change under %v in etcd. %v\n Sleeping 10s...", categoriesKeyPre, err.Error())
			time.Sleep(10 * time.Second)
			continue
		}
		r.redefineCategoryList()
	}
}

func (r *ServiceRegistry) redefineServiceList() {
	infoLogger.Printf("Reloading service list.")
	services := make(map[string]Service)
	servicesResp, err := r.etcd.Get(context.Background(), servicesKeyPre, &client.GetOptions{Sort: true})
	if err != nil {
		errorLogger.Printf("Failed to get value from %v: %v.", servicesKeyPre, err.Error())
		return
	}
	if !servicesResp.Node.Dir {
		errorLogger.Printf("[%v] is not a directory", servicesResp.Node.Key)
		return
	}
	for _, serviceNode := range servicesResp.Node.Nodes {
		if !serviceNode.Dir {
			warnLogger.Printf("[%v] is not a directory", serviceNode.Key)
			continue
		}
		name := filepath.Base(serviceNode.Key)
		path := defaultPath
		pathResp, err := r.etcd.Get(context.Background(), serviceNode.Key+pathSuffix, &client.GetOptions{Sort: true})
		if err != nil {
			warnLogger.Printf("Failed to get health check path from %v: %v. Using default %v", serviceNode.Key, err.Error(), defaultPath)
		} else {
			path = pathResp.Node.Value
		}

		categoriesResp, err := r.etcd.Get(context.Background(), serviceNode.Key+categoriesSuffix, &client.GetOptions{Sort: true})
		var categories []string

		if err != nil || categoriesResp.Node.Dir {
			warnLogger.Printf("Failed to get app category from [%v]: [%v]. Using default 'default'", serviceNode.Key+categoriesSuffix, err.Error())
			categories = append(categories, "default")
		} else {
			categories = strings.Split(categoriesResp.Node.Value, ",")
		}
		services[name] = Service{Name: name, Host: r.vulcandAddr, Path: fmt.Sprintf(pathPre, name, path), Categories: categories}
	}
	r.services = services
	infoLogger.Printf("%v", r.services)
}

func (r *ServiceRegistry) redefineCategoryList() {
	infoLogger.Printf("Reloading category list.")
	categories := make(map[string]Category)
	categoriesResp, err := r.etcd.Get(context.Background(), categoriesKeyPre, &client.GetOptions{Sort: true})
	if err != nil {
		errorLogger.Printf("Failed to get value from %v: %v.", categoriesKeyPre, err.Error())
		return
	}
	if !categoriesResp.Node.Dir {
		errorLogger.Printf("[%v] is not a directory", categoriesResp.Node.Key)
		return
	}
	for _, categoryNode := range categoriesResp.Node.Nodes {
		if !categoryNode.Dir {
			warnLogger.Printf("[%v] is not a directory", categoryNode.Key)
			continue
		}
		name := filepath.Base(categoryNode.Key)
		periodResp, err := r.etcd.Get(context.Background(), categoryNode.Key+periodKeySuffix, &client.GetOptions{Sort: true})
		period := defaultDuration
		if err != nil {
			warnLogger.Printf("Failed to get health check period from %v: %v. Using default %v", categoryNode.Key, err.Error(), defaultDuration)
		} else {
			periodInt, err := strconv.Atoi(periodResp.Node.Value)
			if err != nil {
				warnLogger.Printf("Error reading health check period value '%v'. Using default %v", periodResp.Node.Value, defaultDuration)
				periodInt = int(defaultDuration.Seconds())
			}
			period = time.Duration(periodInt) * time.Second
		}
		resilientResp, err := r.etcd.Get(context.Background(), categoryNode.Key+resilientSuffix, nil)
		resilient := false
		if err != nil {
			warnLogger.Printf("Failed to get resilient setting from %v: %v. Using default: false.\n", categoryNode.Key, err.Error())
		} else {
			resilientBool, err := strconv.ParseBool(resilientResp.Node.Value)
			if err != nil {
				warnLogger.Printf("Error reading resilient setting '%v' at key %v. Using default: false.", resilientResp.Node.Value, resilientResp.Node.Key)
				resilientBool = false
			}
			resilient = resilientBool
		}

		categories[name] = Category{Name: name, Period: period, IsResilient: resilient}
	}
	r.categories = categories
	infoLogger.Printf("%v", r.categories)
}

func (r ServiceRegistry) scheduleCheck(mService *MeasuredService, timer *time.Timer) {
	// wait
	select {
	case <-mService.cachedHealth.terminate:
		return
	case <-timer.C:
	}

	// run check
	healthResult := fthealth.RunCheck(mService.service.Name,
		fmt.Sprintf("Checks the health of %v", mService.service.Name),
		true,
		NewServiceHealthCheck(*mService.service, r.checker))

	// write to cache
	mService.cachedHealth.toWriteToCache <- healthResult

	// write to graphite buffer
	select {
	case mService.bufferedHealths.buffer <- healthResult:
	default:
	}

	waitDuration := r.findShortestPeriod(*mService.service)
	go r.scheduleCheck(mService, time.NewTimer(waitDuration))
}

func (r ServiceRegistry) findShortestPeriod(service Service) time.Duration {
	minSeconds := defaultDuration.Seconds()
	minDuration := defaultDuration
	for _, categoryName := range service.Categories {
		category, ok := r.categories[categoryName]
		if !ok {
			continue
		}
		if category.Period.Seconds() < minSeconds {
			minSeconds = category.Period.Seconds()
			minDuration = category.Period
		}
	}
	return minDuration
}

//returns true, only if all categoryNames are considered resilient.
func (r ServiceRegistry) areResilient(categoryNames []string) bool {
	for _, c := range categoryNames {
		if !r.categories[c].IsResilient {
			return false
		}
	}
	return true
}

func (r ServiceRegistry) matchingCategories(s []string) []string {
	var result []string
	for _, a := range s {
		if _, ok := r.categories[a]; ok {
			result = append(result, a)
		}
	}
	return result
}

func (s Service) String() string {
	return fmt.Sprintf("Service: [%s]. Categories: %v", s.Name, s.Categories)
}

func (m servicesMap) String() string {
	var lines []string
	for _, v := range m {
		lines = append(lines, v.String())
	}
	sort.Strings(lines)
	var result string
	for _, line := range lines {
		result = result + "\t" + line + "\n"
	}
	return fmt.Sprintf("Services: [\n%s]", result)
}
func (c Category) String() string {
	return fmt.Sprintf("Category: [%s]. Period: [%v]. Resilient: [%t]", c.Name, c.Period, c.IsResilient)
}

func (m categoriesMap) String() string {
	var lines []string
	for _, v := range m {
		lines = append(lines, v.String())
	}
	sort.Strings(lines)
	var result string
	for _, line := range lines {
		result = result + "\t" + line + "\n"
	}
	return fmt.Sprintf("Categories: [\n%s]", result)
}
