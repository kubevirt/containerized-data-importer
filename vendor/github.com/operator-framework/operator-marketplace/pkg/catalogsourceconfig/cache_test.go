package catalogsourceconfig_test

import (
	"os"
	"testing"

	marketplace "github.com/operator-framework/operator-marketplace/pkg/apis/operators/v1"
	"github.com/operator-framework/operator-marketplace/pkg/catalogsourceconfig"
	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var cache catalogsourceconfig.Cache
var inPackages []string
var csc *marketplace.CatalogSourceConfig
var testUID types.UID

func TestMain(m *testing.M) {
	cache = catalogsourceconfig.NewCache()

	testUID = types.UID("123")
	inPackages = []string{"foo", "bar"}
	csc = helperNewCatalogSourceConfig(testUID, "target", "foo,bar")
	cache.Set(csc)

	exitCode := m.Run()
	os.Exit(exitCode)
}

// TestGet tests if an CatalogSourceConfig object inserted into the cache is found.
func TestGet(t *testing.T) {
	spec, found := cache.Get(csc)
	outPackages := spec.GetPackageIDs()
	assert.ElementsMatch(t, inPackages, outPackages)
	assert.True(t, found)
}

// Test cache eviction
func TestEviction(t *testing.T) {
	evictcsc := helperNewCatalogSourceConfig("678", "target", "foo,bar")
	cache.Set(evictcsc)
	cache.Evict(evictcsc)
	_, found := cache.Get(evictcsc)
	assert.False(t, found)
}

// TestNotFound asserts that a CatalogSourceConfig object not inserted into
// cache is not found.
func TestNotFound(t *testing.T) {
	_, found := cache.Get(helperNewCatalogSourceConfig(types.UID("456"), "target", "foo,bar"))
	assert.False(t, found)
}

// TestStale tests various combination of cache staleness
func TestStale(t *testing.T) {
	// Test if cache is not stale given a CatalogSourceConfig with the expected
	// packages.
	pkgStale, targetStale := cache.IsEntryStale(csc)
	assert.False(t, pkgStale)
	assert.False(t, targetStale)

	// Test if cache is not stale given a CatalogSourceConfig with the expected
	// packages in different order.
	csc = helperNewCatalogSourceConfig(testUID, "target", "bar,foo")
	pkgStale, targetStale = cache.IsEntryStale(csc)
	assert.False(t, pkgStale)
	assert.False(t, targetStale)

	// Test if cache is stale given a CatalogSourceConfig with new packages.
	csc = helperNewCatalogSourceConfig(testUID, "target", "foo,bar,baz")
	pkgStale, targetStale = cache.IsEntryStale(csc)
	assert.True(t, pkgStale)
	assert.False(t, targetStale)

	// Test if cache is stale given a CatalogSourceConfig with the a new
	// TargetNamespace.
	csc = helperNewCatalogSourceConfig(testUID, "newtarget", "foo,bar,baz")
	pkgStale, targetStale = cache.IsEntryStale(csc)
	assert.True(t, pkgStale)
	assert.True(t, targetStale)
}

func helperNewCatalogSourceConfig(UID types.UID, targetNamespace, packages string) *marketplace.CatalogSourceConfig {
	return &marketplace.CatalogSourceConfig{
		ObjectMeta: metav1.ObjectMeta{
			UID: UID,
		},
		Spec: marketplace.CatalogSourceConfigSpec{
			TargetNamespace: targetNamespace,
			Packages:        packages,
		},
	}
}
