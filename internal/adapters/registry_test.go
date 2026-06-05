package adapters

import (
	"context"
	"testing"

	"github.com/agent-vcr/agent-vcr/internal/trace"
)

type testAdapter struct {
	name string
}

func (a testAdapter) Name() string        { return a.name }
func (a testAdapter) DisplayName() string { return a.name }
func (a testAdapter) Probe(ctx context.Context) (*ProbeResult, error) {
	return &ProbeResult{Found: true}, nil
}
func (a testAdapter) Install(ctx context.Context, opts InstallOptions) error   { return nil }
func (a testAdapter) Uninstall(ctx context.Context, opts InstallOptions) error { return nil }
func (a testAdapter) Normalize(ctx context.Context, raw trace.RawEvent) ([]trace.Event, error) {
	return nil, nil
}
func (a testAdapter) Capabilities() Capabilities { return Capabilities{} }

func TestRegistryGetAndList(t *testing.T) {
	registry := newRegistry()
	registry.Register(testAdapter{name: "zeta"})
	registry.Register(testAdapter{name: "alpha"})

	if _, ok := registry.Get("alpha"); !ok {
		t.Fatal("expected adapter alpha to be registered")
	}

	list := registry.List()
	if got, want := len(list), 2; got != want {
		t.Fatalf("len(List()) = %d, want %d", got, want)
	}
	if got, want := list[0].Name(), "alpha"; got != want {
		t.Fatalf("first adapter = %q, want %q", got, want)
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	registry := newRegistry()
	registry.Register(testAdapter{name: "alpha"})

	defer func() {
		if recover() == nil {
			t.Fatal("expected duplicate registration to panic")
		}
	}()

	registry.Register(testAdapter{name: "alpha"})
}
