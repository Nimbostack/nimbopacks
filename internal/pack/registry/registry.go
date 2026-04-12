// Package registry maintains the global list of registered language packs.
package registry

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/Nimbostack/nimbopacks/internal/pack"
	"github.com/Nimbostack/nimbopacks/internal/types"
)

var (
	mu    sync.RWMutex
	packs = make(map[string]pack.Pack)
	order []string
)

func Register(p pack.Pack) {
	mu.Lock()
	defer mu.Unlock()
	name := p.Name()
	if _, exists := packs[name]; exists {
		panic(fmt.Sprintf("nimbopacks: duplicate pack: %q", name))
	}
	packs[name] = p
	order = append(order, name)
}

func Get(name string) pack.Pack {
	mu.RLock()
	defer mu.RUnlock()
	return packs[name]
}

func All() []pack.Pack {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]pack.Pack, 0, len(order))
	for _, name := range order {
		out = append(out, packs[name])
	}
	return out
}

func Names() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, len(order))
	copy(out, order)
	return out
}

type DetectResult struct {
	Pack   pack.Pack
	Result types.DetectResult
}

func DetectAll(ctx context.Context, srcDir string) ([]DetectResult, error) {
	allPacks := All()
	if len(allPacks) == 0 {
		return nil, errors.New("no packs registered")
	}
	var results []DetectResult
	for _, p := range allPacks {
		res, err := p.Detect(ctx, srcDir)
		if err != nil {
			fmt.Printf("warning: pack %q failed: %v\n", p.Name(), err)
			continue
		}
		if res != nil && res.Confidence > 0 {
			results = append(results, DetectResult{Pack: p, Result: *res})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Result.Confidence > results[j].Result.Confidence
	})
	return results, nil
}

func DetectBest(ctx context.Context, srcDir string) (*DetectResult, error) {
	results, err := DetectAll(ctx, srcDir)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return &results[0], nil
}

func ResetForTesting() {
	mu.Lock()
	defer mu.Unlock()
	packs = make(map[string]pack.Pack)
	order = nil
}
