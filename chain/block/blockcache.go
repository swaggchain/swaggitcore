package block

import (
	"swagg/chain/cache"
)



type BlockCache struct {
	*cache.BaseCache

}

func (c *BlockCache) Add(key string, item *Block) *BlockCache {

	c.BaseCache.R.Set(key, item, 1)
	c.BaseCache.R.Wait()
	return c

}


func (c *BlockCache) Update(key string, b *Block) {

	c.Del(key)
	c.Add(key, b)


}

func (c *BlockCache) Del(key string) {

	c.R.Del(key)

}

func (c *BlockCache) GetByKey(key string) interface{} {

	value, found := c.R.Get(key)
	if !found {
		panic("not found")
	}
	return value

}

