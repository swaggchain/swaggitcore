package common

import (
	"context"
	"time"
)

type Context struct {

	ctx context.Context
	environment string
	Log *Logger
	CreatedAt int64
	Entries map[string]*contextEntry
	Errors []error
	HasErrors bool


}

type contextEntry struct {

	Version uint32
	Name string
	Key []byte
	Value *interface{}
	timestamp time.Time
	ttl time.Duration

}


func CreateNewContext(prod bool) *Context {

	ctx := context.Background()
	c := &Context{
		ctx: ctx,
	}

	if prod != false {
		c.SetProductionMode()
		c.AttachLogger(false)
	} else {
		c.SetDevMode()
		c.AttachLogger(true)
	}

	return c

}

func (c *Context) SetProductionMode() {
	c.environment = "production"
}

func (c *Context) SetDevMode() {
	c.environment = "devmode"
}

func (c *Context) SetCreatedAt() {
	c.CreatedAt = time.Now().UnixNano()
}

func (c *Context) AttachLogger(prodMode bool) {

	c.Log = CreateNewLogger(prodMode)

}