package controller

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
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

// NewCacheAndClient creates a new informer cache and client delegating to that
// cache. The cache can be used to retrieve an informer for any GVK or
// runtime.Object. The client can be used to interact with the apiserver.
func NewCacheAndClient(config *rest.Config, opts ...RuntimeOptionFunc) (cache.Cache, client.Client, error) {
	if config == nil {
		return nil, nil, fmt.Errorf("must specify Config")
	}

	resolvedOpts := &RuntimeOptions{}
	for _, optFunc := range opts {
		optFunc(resolvedOpts)
	}

	mapper, err := apiutil.NewDiscoveryRESTMapper(config)
	if err != nil {
		return nil, nil, err
	}

	cache, err := cache.New(config, cache.Options{Scheme: resolvedOpts.Scheme, Mapper: mapper, Resync: resolvedOpts.Resync, Namespace: resolvedOpts.Namespace})
	if err != nil {
		return nil, nil, err
	}

	apiClient, err := client.New(config, client.Options{Scheme: resolvedOpts.Scheme, Mapper: mapper})
	if err != nil {
		return nil, nil, err
	}

	cachingClient := &client.DelegatingClient{
		Reader: &client.DelegatingReader{
			CacheReader:  cache,
			ClientReader: apiClient,
		},
		Writer:       apiClient,
		StatusClient: apiClient,
	}

	return cache, cachingClient, err
}

// MustGetInformer gets the informer for the given object from the given cache,
// or panics if an error occurs.
func MustGetInformer(cache cache.Cache, obj runtime.Object) toolscache.SharedIndexInformer {
	informer, err := cache.GetInformer(obj)
	if err != nil {
		panic(err)
	}
	return informer
}

// MustAddEventHandler adds the given event handler to the informer for the
// given object in the given cache, or panics if an error occurs.
func MustAddEventHandler(cache cache.Cache, obj runtime.Object, handler toolscache.ResourceEventHandler) {
	MustGetInformer(cache, obj).AddEventHandler(handler)
}
