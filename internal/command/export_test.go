package command

import (
	"go.octolab.org/toolset/pulse/internal/hide"
	"go.octolab.org/toolset/pulse/internal/vault"
	"go.octolab.org/toolset/pulse/internal/watch"
)

// SetLoader replaces what loads the lists for a test and returns the way
// back.
func SetLoader(loader watch.Loader) (restore func()) {
	previous := newLoader
	newLoader = func() watch.Loader { return loader }
	return func() { newLoader = previous }
}

// SetVaultProvider makes the name of [vault] provider answer with the
// provider for a test and returns the way back.
func SetVaultProvider(name string, provider hide.Provider) (restore func()) {
	previous, found := vaultProviders[name]
	vaultProviders[name] = func(vault.Settings) (hide.Provider, error) { return provider, nil }
	return func() {
		if found {
			vaultProviders[name] = previous
		} else {
			delete(vaultProviders, name)
		}
	}
}
