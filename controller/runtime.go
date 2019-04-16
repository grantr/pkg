package controller

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

var (
	// Cache is a global informer cache for use by reconcilers.
	Cache cache.Cache

	cacheOnce sync.Once
	mapper    meta.RESTMapper
)

// RuntimeOptions contains options for creating caches and clients.
type RuntimeOptions struct {
	// Scheme is the scheme to use for mapping objects to GroupVersionKinds.
	// Default is scheme.Scheme from client-go.
	Scheme *runtime.Scheme

	// Resync is the resync period. Default is defaultResyncTime, currently
	// 10 hours.
	Resync *time.Duration

	// Namespace restricts the cache to the given namespace.
	// Default is to watch all namespaces.
	Namespace string
}

// RuntimeOptionFunc is a function that mutates a RuntimeOptions struct. It
// implements the functional options pattern.
type RuntimeOptionFunc func(*RuntimeOptions)

// Scheme is a functional option that sets the Scheme field of a RuntimeOptions
// struct.
func Scheme(scheme *runtime.Scheme) RuntimeOptionFunc {
	return func(o *RuntimeOptions) {
		o.Scheme = scheme
	}
}

// Resync is a functional option that sets the Resync field of a RuntimeOptions
// struct.
func Resync(resync time.Duration) RuntimeOptionFunc {
	return func(o *RuntimeOptions) {
		o.Resync = &resync
	}
}

// Namespace is a functional option that sets the Namespace field of a
// RuntimeOptions struct.
func Namespace(namespace string) RuntimeOptionFunc {
	return func(o *RuntimeOptions) {
		o.Namespace = namespace
	}
}

// StartCache starts the global informer cache. This must be called before
// calling NewClient, MustGetInformer, or MustAddEventHandler.
func StartCache(config *rest.Config, stopCh <-chan struct{}, opts ...RuntimeOptionFunc) error {
	if config == nil {
		return fmt.Errorf("must specify Config")
	}

	resolvedOpts := &RuntimeOptions{}
	for _, optFunc := range opts {
		optFunc(resolvedOpts)
	}

	var err error
	cacheOnce.Do(func() {
		mapper, err = apiutil.NewDiscoveryRESTMapper(config)
		if err != nil {
			return
		}

		Cache, err = cache.New(config, cache.Options{Scheme: resolvedOpts.Scheme, Mapper: mapper, Resync: resolvedOpts.Resync, Namespace: resolvedOpts.Namespace})
		if err != nil {
			return
		}
	})

	return err
}

// NewClient creates a new client for interacting with the apiserver. The client
// delegates reads to the global informer cache. StartCache must be called
// before calling this method.
func NewClient(config *rest.Config, opts ...RuntimeOptionFunc) (client.Client, error) {
	if config == nil {
		return nil, fmt.Errorf("must specify Config")
	}

	resolvedOpts := &RuntimeOptions{}
	for _, optFunc := range opts {
		optFunc(resolvedOpts)
	}

	apiClient, err := client.New(config, client.Options{Scheme: resolvedOpts.Scheme, Mapper: mapper})
	if err != nil {
		return nil, err
	}

	cachingClient := &client.DelegatingClient{
		Reader: &client.DelegatingReader{
			CacheReader:  Cache,
			ClientReader: apiClient,
		},
		Writer:       apiClient,
		StatusClient: apiClient,
	}

	return cachingClient, err
}

// MustGetInformer gets the informer for the given object from the global
// informer cache, or panics if an error occurs. StartCache must be called
// before calling this method.
func MustGetInformer(obj runtime.Object) toolscache.SharedIndexInformer {
	informer, err := Cache.GetInformer(obj)
	if err != nil {
		panic(err)
	}
	return informer
}

// MustAddEventHandler adds the given event handler to the informer for the
// given object in the global cache, or panics if an error occurs. StartCache
// must be called before calling this method.
func MustAddEventHandler(obj runtime.Object, handler toolscache.ResourceEventHandler) {
	MustGetInformer(obj).AddEventHandler(handler)
}
