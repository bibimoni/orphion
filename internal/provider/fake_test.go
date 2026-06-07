package provider

import (
	"context"
	"fmt"
	"testing"
)

type fakeProvider struct {
	name string
}

func (p *fakeProvider) Search(ctx context.Context, query, kind string) ([]Anime, error) {
	return nil, fmt.Errorf("not implemented")
}
func (p *fakeProvider) Episodes(ctx context.Context, animeID string) ([]Episode, error) {
	return nil, fmt.Errorf("not implemented")
}
func (p *fakeProvider) Streams(ctx context.Context, episodeID string) ([]Stream, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestRegistry_Register(t *testing.T) {
	reg := NewRegistry()
	p := &fakeProvider{name: "test"}
	if err := reg.Register("test", p); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := reg.Register("test", p); err == nil {
		t.Fatal("expected duplicate key error")
	}
	if err := reg.Register("dupe", p); err != nil {
		t.Fatal(err)
	}
}

func TestRegistry_Get(t *testing.T) {
	reg := NewRegistry()
	prov := &fakeProvider{name: "test"}
	reg.Register("test", prov) //nolint:errcheck // test registration cannot fail

	got, err := reg.Get("test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != prov {
		t.Fatal("Get() returned different provider")
	}

	_, err = reg.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Get("unknown")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
