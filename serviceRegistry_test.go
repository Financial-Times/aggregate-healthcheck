package main

import (
	"testing"

	"github.com/coreos/etcd/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/context"
)

type MockEtcdKeysAPI struct {
	mock.Mock
}

func (etcd MockEtcdKeysAPI) Get(ctx context.Context, key string, opts *client.GetOptions) (*client.Response, error) {
	args := etcd.Called()
	return args.Get(0).(*client.Response), args.Error(1)
}

func (etcd MockEtcdKeysAPI) Set(ctx context.Context, key, value string, opts *client.SetOptions) (*client.Response, error) {
	args := etcd.Called()
	return args.Get(0).(*client.Response), args.Error(1)
}

func (etcd MockEtcdKeysAPI) Delete(ctx context.Context, key string, opts *client.DeleteOptions) (*client.Response, error) {
	args := etcd.Called()
	return args.Get(0).(*client.Response), args.Error(1)
}

func (etcd MockEtcdKeysAPI) Create(ctx context.Context, key, value string) (*client.Response, error) {
	args := etcd.Called()
	return args.Get(0).(*client.Response), args.Error(1)
}

func (etcd MockEtcdKeysAPI) CreateInOrder(ctx context.Context, dir, value string, opts *client.CreateInOrderOptions) (*client.Response, error) {
	args := etcd.Called()
	return args.Get(0).(*client.Response), args.Error(1)
}

func (etcd MockEtcdKeysAPI) Update(ctx context.Context, key, value string) (*client.Response, error) {
	args := etcd.Called()
	return args.Get(0).(*client.Response), args.Error(1)
}

func (etcd MockEtcdKeysAPI) Watcher(key string, opts *client.WatcherOptions) client.Watcher {
	args := etcd.Called()
	return args.Get(0).(client.Watcher)
}

func TestRedefineCategoryListEmpty(t *testing.T) {
	any := func(x interface{}) bool { return true }

	categoryNodes := client.Nodes{}
	categoryFolder := client.Node{Key: "/ft/healthcheck-categories", Dir: true, Nodes: categoryNodes}

	etcd := new(MockEtcdKeysAPI)
	response := client.Response{Node: &categoryFolder}
	etcd.On("Get", mock.MatchedBy(any), mock.MatchedBy(any), mock.MatchedBy(any)).Return(&response, nil)

	registry := NewCocoServiceRegistry(etcd, "127.0.0.1", nil)

	registry.redefineCategoryList()

	actual := registry.categories()
	_, defCategoryPresent := actual["default"]
	assert.True(t, defCategoryPresent, "default category should be present")
}

func TestRedefineCategoryList(t *testing.T) {
	any := func(x interface{}) bool { return true }

	fooCategory := client.Node{Key: "/ft/healthcheck-categories/foo", Dir: true, Nodes: client.Nodes{}}
	categoryNodes := client.Nodes{&fooCategory}
	categoryFolder := client.Node{Key: "/ft/healthcheck-categories", Dir: true, Nodes: categoryNodes}

	etcd := new(MockEtcdKeysAPI)
	response := client.Response{Node: &categoryFolder}
	etcd.On("Get", mock.MatchedBy(any), mock.MatchedBy(any), mock.MatchedBy(any)).Return(&response, nil)

	registry := NewCocoServiceRegistry(etcd, "127.0.0.1", nil)

	registry.redefineCategoryList()

	actual := registry.categories()
	assert.Len(t, actual, 2, "category list")
	_, categoryPresent := actual["default"]
	assert.True(t, categoryPresent, "default category should be present")
	_, categoryPresent = actual["foo"]
	assert.True(t, categoryPresent, "foo category should be present")
}
