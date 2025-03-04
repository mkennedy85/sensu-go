package graphql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/graph-gophers/dataloader"
	corev2 "github.com/sensu/sensu-go/api/core/v2"
	"github.com/sensu/sensu-go/backend/authorization"
	"github.com/sensu/sensu-go/backend/store"
)

type key int

const (
	loadersKey key = iota
	assetsLoaderKey
	checkConfigsLoaderKey
	entitiesLoaderKey
	eventsLoaderKey
	eventFiltersLoaderKey
	handlersLoaderKey
	mutatorsLoaderKey
	namespacesLoaderKey
	silencedsLoaderKey

	// chunk size used by dataloader when retrieving resources from the store
	loaderPageSize = 250

	// the maximum number of records that will be read from the store by the
	// dataloader; too many puts significant strain on memory.
	maxLengthEntityDataloader  = 1_000
	maxLengthEventDataloader   = 1_000
	maxLengthGenericDataloader = 2_500
)

var (
	errLoadersNotFound        = errors.New("loaders was not found inside context")
	errLoaderNotFound         = errors.New("loader was not found")
	errUnexpectedLoaderResult = errors.New("loader returned unexpected result")
)

// assets

func loadAssetsBatchFn(c AssetClient) dataloader.BatchFunc {
	return func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
		results := make([]*dataloader.Result, 0, len(keys))
		for _, key := range keys {
			ctx := store.NamespaceContext(ctx, key.String())
			ctx = context.WithValue(ctx, corev2.PageSizeKey, maxLengthGenericDataloader)
			records, err := c.ListAssets(ctx)
			result := &dataloader.Result{Data: records, Error: handleListErr(err)}
			results = append(results, result)
		}
		return results
	}
}

func loadAssets(ctx context.Context, ns string) ([]*corev2.Asset, error) {
	var records []*corev2.Asset
	loader, err := getLoader(ctx, assetsLoaderKey)
	if err != nil {
		return records, err
	}

	results, err := loader.Load(ctx, dataloader.StringKey(ns))()
	records, ok := results.([]*corev2.Asset)
	if err == nil && !ok {
		err = fmt.Errorf("asset loader: %s", errUnexpectedLoaderResult)
	}
	return records, err
}

// checks

func loadCheckConfigsBatchFn(c CheckClient) dataloader.BatchFunc {
	return func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
		results := make([]*dataloader.Result, 0, len(keys))
		for _, key := range keys {
			ctx := store.NamespaceContext(ctx, key.String())
			ctx = context.WithValue(ctx, corev2.PageSizeKey, maxLengthGenericDataloader)
			records, err := c.ListChecks(ctx)
			result := &dataloader.Result{Data: records, Error: handleListErr(err)}
			results = append(results, result)
		}
		return results
	}
}

func loadCheckConfigs(ctx context.Context, ns string) ([]*corev2.CheckConfig, error) {
	var records []*corev2.CheckConfig
	loader, err := getLoader(ctx, checkConfigsLoaderKey)
	if err != nil {
		return records, err
	}

	results, err := loader.Load(ctx, dataloader.StringKey(ns))()
	records, ok := results.([]*corev2.CheckConfig)
	if err == nil && !ok {
		err = fmt.Errorf("check loader: %s", errUnexpectedLoaderResult)
	}
	return records, err
}

// entities

func listEntities(ctx context.Context, c EntityClient, maxSize int) (records []*corev2.Entity, err error) {
	pred := &store.SelectionPredicate{Continue: "", Limit: int64(loaderPageSize)}
	for {
		r, err := c.ListEntities(ctx, pred)
		if err != nil {
			return records, err
		}
		records = append(records, r...)
		if pred.Continue == "" || len(r) < loaderPageSize || len(records) >= maxSize {
			break
		}
	}
	return
}

func loadEntitiesBatchFn(c EntityClient) dataloader.BatchFunc {
	return func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
		results := make([]*dataloader.Result, 0, len(keys))
		for _, key := range keys {
			ctx := store.NamespaceContext(ctx, key.String())
			records, err := listEntities(ctx, c, maxLengthEntityDataloader)
			result := &dataloader.Result{Data: records, Error: handleListErr(err)}
			results = append(results, result)
		}
		return results
	}
}

func loadEntities(ctx context.Context, ns string) ([]*corev2.Entity, error) {
	var records []*corev2.Entity
	loader, err := getLoader(ctx, entitiesLoaderKey)
	if err != nil {
		return records, err
	}

	results, err := loader.Load(ctx, dataloader.StringKey(ns))()
	records, ok := results.([]*corev2.Entity)
	if err == nil && !ok {
		err = fmt.Errorf("entity loader: %s", errUnexpectedLoaderResult)
	}
	return records, err
}

// events

type eventCacheKey struct {
	namespace string
	entity    string
}

func newEventCacheKey(key string) *eventCacheKey {
	els := strings.SplitN(key, "\n", 2)
	return &eventCacheKey{namespace: els[0], entity: els[1]}
}

func (k *eventCacheKey) String() string {
	return strings.Join([]string{k.namespace, k.entity}, "\n")
}

func (k *eventCacheKey) Raw() interface{} {
	return k
}

func listEvents(ctx context.Context, c EventClient, entity string, maxSize int) ([]*corev2.Event, error) {
	pred := &store.SelectionPredicate{Continue: "", Limit: int64(loaderPageSize)}
	list := func(ctx context.Context, entity string, pred *store.SelectionPredicate) ([]*corev2.Event, error) {
		if entity == "" {
			return c.ListEvents(ctx, pred)
		}
		return c.ListEventsByEntity(ctx, entity, pred)
	}
	results := []*corev2.Event{}
	for {
		r, err := list(ctx, entity, pred)
		if err != nil {
			return results, err
		}
		results = append(results, r...)
		if pred.Continue == "" || len(r) < loaderPageSize || len(results) >= maxSize {
			break
		}
	}
	return results, nil
}

func loadEventsBatchFn(c EventClient) dataloader.BatchFunc {
	return func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
		results := make([]*dataloader.Result, 0, len(keys))
		for _, key := range keys {
			key := newEventCacheKey(key.String())
			ctx := store.NamespaceContext(ctx, key.namespace)
			records, err := listEvents(ctx, c, key.entity, maxLengthEventDataloader)
			result := &dataloader.Result{Data: records, Error: handleListErr(err)}
			results = append(results, result)
		}
		return results
	}
}

func loadEvents(ctx context.Context, ns, entity string) ([]*corev2.Event, error) {
	var records []*corev2.Event
	loader, err := getLoader(ctx, eventsLoaderKey)
	if err != nil {
		return records, err
	}

	key := &eventCacheKey{namespace: ns, entity: entity}
	results, err := loader.Load(ctx, key)()
	records, ok := results.([]*corev2.Event)
	if err == nil && !ok {
		err = fmt.Errorf("event loader: %s", errUnexpectedLoaderResult)
	}
	return records, err
}

// event filters

func loadEventFiltersBatchFn(c EventFilterClient) dataloader.BatchFunc {
	return func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
		results := make([]*dataloader.Result, 0, len(keys))
		for _, key := range keys {
			ctx := store.NamespaceContext(ctx, key.String())
			ctx = context.WithValue(ctx, corev2.PageSizeKey, maxLengthGenericDataloader)
			records, err := c.ListEventFilters(ctx)
			result := &dataloader.Result{Data: records, Error: handleListErr(err)}
			results = append(results, result)
		}
		return results
	}
}

func loadEventFilters(ctx context.Context, ns string) ([]*corev2.EventFilter, error) {
	var records []*corev2.EventFilter
	loader, err := getLoader(ctx, eventFiltersLoaderKey)
	if err != nil {
		return records, err
	}

	results, err := loader.Load(ctx, dataloader.StringKey(ns))()
	records, ok := results.([]*corev2.EventFilter)
	if err == nil && !ok {
		err = fmt.Errorf("filter loader: %s", errUnexpectedLoaderResult)
	}
	return records, err
}

// handlers

func loadHandlersBatchFn(c HandlerClient) dataloader.BatchFunc {
	return func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
		results := make([]*dataloader.Result, 0, len(keys))
		for _, key := range keys {
			ctx := store.NamespaceContext(ctx, key.String())
			ctx = context.WithValue(ctx, corev2.PageSizeKey, maxLengthGenericDataloader)
			records, err := c.ListHandlers(ctx)
			result := &dataloader.Result{Data: records, Error: handleListErr(err)}
			results = append(results, result)
		}
		return results
	}
}

func loadHandlers(ctx context.Context, ns string) ([]*corev2.Handler, error) {
	var records []*corev2.Handler
	loader, err := getLoader(ctx, handlersLoaderKey)
	if err != nil {
		return records, err
	}

	results, err := loader.Load(ctx, dataloader.StringKey(ns))()
	records, ok := results.([]*corev2.Handler)
	if err == nil && !ok {
		err = fmt.Errorf("handler loader: %s", errUnexpectedLoaderResult)
	}
	return records, err
}

// mutators

func loadMutatorsBatchFn(c MutatorClient) dataloader.BatchFunc {
	return func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
		results := make([]*dataloader.Result, 0, len(keys))
		for _, key := range keys {
			ctx := store.NamespaceContext(ctx, key.String())
			ctx = context.WithValue(ctx, corev2.PageSizeKey, maxLengthGenericDataloader)
			records, err := c.ListMutators(ctx)
			result := &dataloader.Result{Data: records, Error: handleListErr(err)}
			results = append(results, result)
		}
		return results
	}
}

func loadMutators(ctx context.Context, ns string) ([]*corev2.Mutator, error) {
	var records []*corev2.Mutator
	loader, err := getLoader(ctx, mutatorsLoaderKey)
	if err != nil {
		return records, err
	}

	results, err := loader.Load(ctx, dataloader.StringKey(ns))()
	records, ok := results.([]*corev2.Mutator)
	if err == nil && !ok {
		err = fmt.Errorf("mutator loader: %s", errUnexpectedLoaderResult)
	}
	return records, err
}

// namespaces

func loadNamespacesBatchFn(c NamespaceClient) dataloader.BatchFunc {
	return func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
		results := make([]*dataloader.Result, 0, len(keys))
		for range keys {
			ctx := context.WithValue(ctx, corev2.PageSizeKey, maxLengthGenericDataloader)
			records, err := c.ListNamespaces(ctx, &store.SelectionPredicate{})
			result := &dataloader.Result{Data: records, Error: handleListErr(err)}
			results = append(results, result)
		}
		return results
	}
}

func loadNamespaces(ctx context.Context) ([]*corev2.Namespace, error) {
	var records []*corev2.Namespace
	loader, err := getLoader(ctx, namespacesLoaderKey)
	if err != nil {
		return records, err
	}

	results, err := loader.Load(ctx, dataloader.StringKey("*"))()
	records, ok := results.([]*corev2.Namespace)
	if err == nil && !ok {
		err = fmt.Errorf("namespace loader: %s", errUnexpectedLoaderResult)
	}
	return records, err
}

// silences

func loadSilencedsBatchFn(c SilencedClient) dataloader.BatchFunc {
	return func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
		results := make([]*dataloader.Result, 0, len(keys))
		for _, key := range keys {
			ctx := store.NamespaceContext(ctx, key.String())
			ctx = context.WithValue(ctx, corev2.PageSizeKey, maxLengthGenericDataloader)
			records, err := c.ListSilenced(ctx)
			result := &dataloader.Result{Data: records, Error: handleListErr(err)}
			results = append(results, result)
		}
		return results
	}
}

func loadSilenceds(ctx context.Context, ns string) ([]*corev2.Silenced, error) {
	var records []*corev2.Silenced
	loader, err := getLoader(ctx, silencedsLoaderKey)
	if err != nil {
		return records, err
	}

	results, err := loader.Load(ctx, dataloader.StringKey(ns))()
	records, ok := results.([]*corev2.Silenced)
	if err == nil && !ok {
		err = fmt.Errorf("silenced loader: %s", errUnexpectedLoaderResult)
	}
	return records, err
}

func contextWithLoaders(ctx context.Context, cfg ServiceConfig, opts ...dataloader.Option) context.Context {
	// Currently all fields are resolved serially, as such we disable batching and
	// rely only on dataloader's cache.
	opts = append([]dataloader.Option{dataloader.WithBatchCapacity(1)}, opts...)

	loaders := map[key]*dataloader.Loader{}
	loaders[assetsLoaderKey] = dataloader.NewBatchedLoader(loadAssetsBatchFn(cfg.AssetClient), opts...)
	loaders[checkConfigsLoaderKey] = dataloader.NewBatchedLoader(loadCheckConfigsBatchFn(cfg.CheckClient), opts...)
	loaders[entitiesLoaderKey] = dataloader.NewBatchedLoader(loadEntitiesBatchFn(cfg.EntityClient), opts...)
	loaders[eventsLoaderKey] = dataloader.NewBatchedLoader(loadEventsBatchFn(cfg.EventClient), opts...)
	loaders[eventFiltersLoaderKey] = dataloader.NewBatchedLoader(loadEventFiltersBatchFn(cfg.EventFilterClient), opts...)
	loaders[handlersLoaderKey] = dataloader.NewBatchedLoader(loadHandlersBatchFn(cfg.HandlerClient), opts...)
	loaders[mutatorsLoaderKey] = dataloader.NewBatchedLoader(loadMutatorsBatchFn(cfg.MutatorClient), opts...)
	loaders[namespacesLoaderKey] = dataloader.NewBatchedLoader(loadNamespacesBatchFn(cfg.NamespaceClient), opts...)
	loaders[silencedsLoaderKey] = dataloader.NewBatchedLoader(loadSilencedsBatchFn(cfg.SilencedClient), opts...)
	return context.WithValue(ctx, loadersKey, loaders)
}

func getLoader(ctx context.Context, loaderKey key) (*dataloader.Loader, error) {
	loaders, ok := ctx.Value(loadersKey).(map[key]*dataloader.Loader)
	if !ok {
		return nil, errLoadersNotFound
	}

	loader, ok := loaders[loaderKey]
	if !ok {
		return loader, errLoaderNotFound
	}
	return loader, nil
}

// When resolving a field, GraphQL does not consider the absence of a value an
// error; as such we omit the error if the API client returns Permission denied.
func handleListErr(err error) error {
	if err == authorization.ErrUnauthorized || err == authorization.ErrNoClaims {
		logger.WithError(err).Warn("couldn't access resource")
		return nil
	}
	return err
}

// When resolving a field, GraphQL does not consider the absence of a value an
// error; as such we omit the error when the API client returns NotFound or
// Permission denied.
func handleFetchResult(resource interface{}, err error) (interface{}, error) {
	if err == authorization.ErrUnauthorized || err == authorization.ErrNoClaims {
		logger.WithError(err).Warn("couldn't access resource")
		return nil, nil
	}
	if _, ok := err.(*store.ErrNotFound); ok {
		logger.WithError(err).Warn("couldn't access resource")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return resource, err
}
