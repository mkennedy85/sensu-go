// +build integration,!race

package etcd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEtcd(t *testing.T) {
	e, cleanup := NewTestEtcd(t)
	defer cleanup()

	client := e.NewEmbeddedClient()

	putsResp, err := client.Put(context.Background(), "key", "value")
	assert.NoError(t, err)
	assert.NotNil(t, putsResp)

	if putsResp == nil {
		assert.FailNow(t, "got nil put response from etcd")
	}

	getResp, err := client.Get(context.Background(), "key")
	assert.NoError(t, err)
	assert.NotNil(t, getResp)

	if getResp == nil {
		assert.FailNow(t, "got nil get response from etcd")
	}
	assert.Equal(t, 1, len(getResp.Kvs))
	assert.Equal(t, "key", string(getResp.Kvs[0].Key))
	assert.Equal(t, "value", string(getResp.Kvs[0].Value))

	require.NoError(t, e.Shutdown())
}

func TestEtcdHealthy(t *testing.T) {
	e, cleanup := NewTestEtcd(t)
	defer cleanup()
	health := e.Healthy()
	assert.True(t, health)
}

func TestGetClientURLs(t *testing.T) {
	etcd, cleanup := NewTestEtcd(t)
	defer cleanup()

	clientURLs := etcd.GetClientURLs()
	if got, want := len(clientURLs), 1; got < want {
		t.Fatalf("got %d client URLs, want at least %d", got, want)
	}
}
