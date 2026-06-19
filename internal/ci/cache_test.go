package ci

import (
	"testing"
)

func TestCacheKey_Deterministic(t *testing.T) {
	cache := NewCache(t.TempDir())
	step := Step{ID: "test-step", Run: "go test ./..."}
	key1 := cache.Key("job1", step, "os=linux;", "matrixhash1")
	key2 := cache.Key("job1", step, "os=linux;", "matrixhash1")
	if key1 != key2 {
		t.Errorf("keys should be identical: %s vs %s", key1, key2)
	}
}

func TestCacheKey_DifferentJobs(t *testing.T) {
	cache := NewCache(t.TempDir())
	step := Step{ID: "test-step", Run: "go test ./..."}
	key1 := cache.Key("job1", step, "os=linux;", "mh1")
	key2 := cache.Key("job2", step, "os=linux;", "mh1")
	if key1 == key2 {
		t.Error("keys for different jobs should differ")
	}
}

func TestCacheHitAndStore(t *testing.T) {
	cache := NewCache(t.TempDir())
	step := Step{ID: "s", Run: "echo hello"}
	key := cache.Key("j", step, "arch=amd64;", "mh")

	if _, ok := cache.Hit(key); ok {
		t.Error("unexpected cache hit before store")
	}

	if err := cache.Store(key, "world"); err != nil {
		t.Fatal(err)
	}

	val, ok := cache.Hit(key)
	if !ok {
		t.Fatal("expected cache hit after store")
	}

	if val != "world" {
		t.Errorf("expected 'world', got '%s'", val)
	}
}

func TestFlatten_Deterministic(t *testing.T) {
	vars := map[string]string{"os": "linux", "arch": "arm64"}
	out1 := flatten(vars)
	out2 := flatten(vars)

	if out1 != out2 {
		t.Errorf("flatten should be deterministic: %s vs %s", out1, out2)
	}

	if out1 != "arch=arm64;os=linux;" {
		t.Errorf("unexpected flatten output: %s", out1)
	}
}
