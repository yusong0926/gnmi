/*
Copyright 2017 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package client implements a fake client implementation to be used with
// streaming telemetry collection.  It provides a simple Updates queue of data
// to send it should be used to provide an RPC free test infra for user facing
// libraries.
package client

import (
	log "github.com/golang/glog"
	"context"
	"github.com/openconfig/gnmi/client"
)

// New can be replaced for any negative testing you would like to do as well.
var New = func(ctx context.Context, q client.Query) (client.Impl, error) {
	return &Client{
		Handler: q.NotificationHandler,
	}, nil
}

// Client is the fake of a client implementation. It will provide a simple
// list of updates to send to the generic client.
//
// The Updates slice can be:
// - client.Notification: passed to query.NotificationHandler
// - error: returned from Recv, interrupts the update stream
// - Block: pauses Recv, proceeds to next update on Unblock
//
// See ExampleClient for sample use case.
type Client struct {
	currUpdate int
	Updates    []interface{}
	Handler    client.NotificationHandler
	// When BlockAfterSync is set, Client will read from it in Recv after
	// sending all Updates before returning ErrStopReading.
	// BlockAfterSync is closed when Close is called.
	BlockAfterSync chan struct{}
	connected      bool
}

// Reset will reset the client to start playing new updates.
func (c *Client) Reset(u []interface{}) {
	c.currUpdate = 0
	c.Updates = u
}

// Recv will be called for each update the generic client wants to receive.
func (c *Client) Recv() error {
	if !c.connected {
		c.Handler(client.Connected{})
		c.connected = true
	}

	for c.currUpdate < len(c.Updates) {
		u := c.Updates[c.currUpdate]
		c.currUpdate++
		log.Infof("fake client update: %v", u)
		switch v := u.(type) {
		case client.Notification:
			c.Handler(v)
			return nil
		case error:
			return v
		case Block:
			<-v
		}
	}

	c.Handler(client.Sync{})
	// We went through all c.Update items.
	if c.BlockAfterSync != nil {
		log.Info("No more updates, blocking on BlockAfterSync")
		<-c.BlockAfterSync
	}
	log.Infof("Recv() returning %v", client.ErrStopReading)
	return client.ErrStopReading
}

// Close is a noop in the fake.
func (c *Client) Close() error {
	if c.BlockAfterSync != nil {
		close(c.BlockAfterSync)
	}
	return nil
}

// Poll is a noop in the fake.
func (c *Client) Poll() error {
	return nil
}

// Set is not supported in fake.
func (c *Client) Set(context.Context, client.SetRequest) (client.SetResponse, error) {
	return client.SetResponse{}, client.ErrUnsupported
}

// Block is a special update that lets the stream of updates to be paused.
// See Client docs for usage example.
type Block chan struct{}

// Unblock unpauses the update stream following the Block. Can only be called
// once.
func (b Block) Unblock() { close(b) }
