package cache

import (
"github.com/dgraph-io/ristretto"

)

type BaseCache struct {
	Name string
	R *ristretto.Cache

}

func CreateNewCache(name string) *BaseCache {

	bc := &BaseCache{}
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e10,
		MaxCost: 4 << 30,
		BufferItems: 256,
	})
	if err != nil {
		panic(err)
	}

	bc.Name = name
	bc.R = cache

	return bc


}