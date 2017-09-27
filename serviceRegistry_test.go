package main

import (
	"errors"
	"os"
	"testing"
	"time"

	"context"

	"github.com/coreos/etcd/client"
	"github.com/stretchr/testify/assert"
)

type TestEtcdKeysAPI struct {
	response *client.Response
	watcher  client.Watcher
}

func (etcd TestEtcdKeysAPI) Get(ctx context.Context, key string, opts *client.GetOptions) (*client.Response, error) {
	return etcd.response, nil
}

func (etcd TestEtcdKeysAPI) Set(ctx context.Context, key, value string, opts *client.SetOptions) (*client.Response, error) {
	return etcd.response, nil
}

func (etcd TestEtcdKeysAPI) Watcher(key string, opts *client.WatcherOptions) client.Watcher {
	return etcd.watcher
}

type TestWatcher struct {
	response *client.Response
}

func (w *TestWatcher) Next(ctx context.Context) (*client.Response, error) {
	if w.response != nil {
		t := w.response
		w.response = nil
		return t, nil
	}

	return nil, errors.New("test etcd watcher error")
}

func TestRedefineCategoryListEmpty(t *testing.T) {
	categoryNodes := client.Nodes{}
	categoryFolder := client.Node{Key: "/ft/healthcheck-categories", Dir: true, Nodes: categoryNodes}

	etcd := TestEtcdKeysAPI{&client.Response{Node: &categoryFolder}, nil}

	registry := NewCocoServiceRegistry(etcd, "127.0.0.1", nil, "test")

	registry.redefineCategoryList()

	actual := registry.categories()
	_, defCategoryPresent := actual["default"]
	assert.True(t, defCategoryPresent, "default category should be present")
}

func TestRedefineCategoryList(t *testing.T) {
	fooCategory := client.Node{Key: "/ft/healthcheck-categories/foo", Dir: true, Nodes: client.Nodes{}}
	categoryNodes := client.Nodes{&fooCategory}
	categoryFolder := client.Node{Key: "/ft/healthcheck-categories", Dir: true, Nodes: categoryNodes}

	etcd := TestEtcdKeysAPI{&client.Response{Node: &categoryFolder}, nil}

	registry := NewCocoServiceRegistry(etcd, "127.0.0.1", nil, "test")

	registry.redefineCategoryList()

	actual := registry.categories()
	assert.Len(t, actual, 2, "category list")
	_, categoryPresent := actual["default"]
	assert.True(t, categoryPresent, "default category should be present")
	_, categoryPresent = actual["foo"]
	assert.True(t, categoryPresent, "foo category should be present")
}

func TestWatchCategories(t *testing.T) {
	initLogs(os.Stdout, os.Stdout, os.Stderr)

	fooCategory := client.Node{Key: "/ft/healthcheck-categories/foo", Dir: true, Nodes: client.Nodes{}}
	categoryNodes := client.Nodes{&fooCategory}
	categoryFolder := client.Node{Key: "/ft/healthcheck-categories", Dir: true, Nodes: categoryNodes}

	etcd := TestEtcdKeysAPI{&client.Response{Node: &categoryFolder}, &TestWatcher{&client.Response{Node: &categoryFolder}}}

	registry := NewCocoServiceRegistry(etcd, "127.0.0.1", nil, "test")
	registry.etcdInterval = time.Second

	go registry.watchCategories()
	time.Sleep(2 * time.Second)

	actual := registry.categories()

	assert.Len(t, actual, 2, "category list")
	_, categoryPresent := actual["default"]
	assert.True(t, categoryPresent, "default category should be present")
	_, categoryPresent = actual["foo"]
	assert.True(t, categoryPresent, "foo category should be present")
}
