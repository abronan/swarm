package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func makeEtcdClient(t *testing.T) Store {
	client := "localhost:4001"

	// Initialize a new store with consul
	kv, err := NewStore(
		ETCD,
		[]string{client},
		&Config{
			ConnectionTimeout: 10 * time.Second,
			EphemeralTTL:      2 * time.Second,
		},
	)
	if err != nil {
		t.Fatalf("cannot create store: %v", err)
	}

	return kv
}

func TestEtcdPutGetDelete(t *testing.T) {
	kv := makeEtcdClient(t)

	key := "foo"
	value := []byte("bar")

	// Put the key
	err := kv.Put(key, value, nil)
	assert.NoError(t, err)

	// Get should return the value and an incremented index
	pair, err := kv.Get(key)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, value)
	assert.NotEqual(t, pair.LastIndex, 0)

	// Delete the key
	err = kv.Delete(key)
	assert.NoError(t, err)

	// Get should fail
	pair, err = kv.Get(key)
	assert.Error(t, err)
	assert.Nil(t, pair)
}

func TestEtcdWatch(t *testing.T) {
	// TODO
}

func TestEtcdAtomicPut(t *testing.T) {
	kv := makeEtcdClient(t)

	key := "hello"
	value := []byte("world")

	// Put the key
	err := kv.Put(key, value, nil)
	assert.NoError(t, err)

	// Get should return the value and an incremented index
	pair, err := kv.Get(key)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, value)
	assert.NotEqual(t, pair.LastIndex, 0)

	// This CAS should succeed
	success, _, err := kv.AtomicPut("hello", []byte("WORLD"), pair, nil)
	assert.NoError(t, err)
	assert.True(t, success)

	// This CAS should fail
	pair.LastIndex = 0
	success, _, err = kv.AtomicPut("hello", []byte("WORLDWORLD"), pair, nil)
	assert.Error(t, err)
	assert.False(t, success)
}

func TestEtcdAtomicDelete(t *testing.T) {
	kv := makeEtcdClient(t)

	key := "hello"
	value := []byte("world")

	// Put the key
	err := kv.Put(key, value, nil)
	assert.NoError(t, err)

	// Get should return the value and an incremented index
	pair, err := kv.Get(key)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, value)
	assert.NotEqual(t, pair.LastIndex, 0)

	tempIndex := pair.LastIndex

	// AtomicDelete should fail
	pair.LastIndex = 0
	success, err := kv.AtomicDelete(key, pair)
	assert.Error(t, err)
	assert.False(t, success)

	// AtomicDelete should succeed
	pair.LastIndex = tempIndex
	success, err = kv.AtomicDelete(key, pair)
	assert.NoError(t, err)
	assert.True(t, success)
}

func TestEtcdLockUnlock(t *testing.T) {
	t.Parallel()
	kv := makeEtcdClient(t)

	key := "foo"
	value := []byte("bar")

	// We should be able to create a new lock on key
	lock, err := kv.NewLock(key, &LockOptions{Value: value})
	assert.NoError(t, err)
	assert.NotNil(t, lock)

	// Lock should successfully succeed or block
	lockChan, err := lock.Lock()
	assert.NoError(t, err)
	assert.NotNil(t, lockChan)

	// Get should work
	pair, err := kv.Get(key)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, value)
	assert.NotEqual(t, pair.LastIndex, 0)

	// Unlock should succeed
	err = lock.Unlock()
	assert.NoError(t, err)

	// Get should work
	pair, err = kv.Get(key)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, value)
	assert.NotEqual(t, pair.LastIndex, 0)
}

func TestEtcdPutEphemeral(t *testing.T) {
	kv := makeEtcdClient(t)

	firstKey := "foo"
	firstValue := []byte("foo")

	secondKey := "bar"
	secondValue := []byte("bar")

	// Put the first key with the Ephemeral flag
	err := kv.Put(firstKey, firstValue, &WriteOptions{Ephemeral: true})
	assert.NoError(t, err)

	// Put a second key with the Ephemeral flag
	err = kv.Put(secondKey, secondValue, &WriteOptions{Ephemeral: true})
	assert.NoError(t, err)

	// Get on firstKey should work
	pair, err := kv.Get(firstKey)
	assert.NoError(t, err)
	assert.NotNil(t, pair)

	// Get on secondKey should work
	pair, err = kv.Get(secondKey)
	assert.NoError(t, err)
	assert.NotNil(t, pair)

	// Let the session expire
	time.Sleep(3 * time.Second)

	// Get on firstKey shouldn't work
	pair, err = kv.Get(firstKey)
	assert.Error(t, err)
	assert.Nil(t, pair)

	// Get on secondKey shouldn't work
	pair, err = kv.Get(secondKey)
	assert.Error(t, err)
	assert.Nil(t, pair)
}

func TestEtcdList(t *testing.T) {
	kv := makeEtcdClient(t)

	prefix := "nodes/"

	firstKey := "nodes/first"
	firstValue := []byte("first")

	secondKey := "nodes/second"
	secondValue := []byte("second")

	// Put the first key
	err := kv.Put(firstKey, firstValue, nil)
	assert.NoError(t, err)

	// Put the second key
	err = kv.Put(secondKey, secondValue, nil)
	assert.NoError(t, err)

	// List should work and return the two correct values
	pairs, err := kv.List(prefix)
	assert.NoError(t, err)
	if assert.NotNil(t, pairs) {
		assert.Equal(t, len(pairs), 2)
	}

	// Check pairs, those are not necessarily in Put order
	for _, pair := range pairs {
		if pair.Key == firstKey {
			assert.Equal(t, pair.Value, firstValue)
		}
		if pair.Key == secondKey {
			assert.Equal(t, pair.Value, secondValue)
		}
	}
}

func TestEtcdDeleteTree(t *testing.T) {
	kv := makeEtcdClient(t)

	prefix := "nodes/"

	firstKey := "nodes/first"
	firstValue := []byte("first")

	secondKey := "nodes/second"
	secondValue := []byte("second")

	// Put the first key
	err := kv.Put(firstKey, firstValue, nil)
	assert.NoError(t, err)

	// Put the second key
	err = kv.Put(secondKey, secondValue, nil)
	assert.NoError(t, err)

	// Get should work on the first Key
	pair, err := kv.Get(firstKey)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, firstValue)
	assert.NotEqual(t, pair.LastIndex, 0)

	// Get should work on the second Key
	pair, err = kv.Get(secondKey)
	assert.NoError(t, err)
	if assert.NotNil(t, pair) {
		assert.NotNil(t, pair.Value)
	}
	assert.Equal(t, pair.Value, secondValue)
	assert.NotEqual(t, pair.LastIndex, 0)

	// Delete Values under directory `nodes`
	err = kv.DeleteTree(prefix)
	assert.NoError(t, err)

	// Get should fail on both keys
	pair, err = kv.Get(firstKey)
	assert.Error(t, err)
	assert.Nil(t, pair)

	pair, err = kv.Get(secondKey)
	assert.Error(t, err)
	assert.Nil(t, pair)
}
