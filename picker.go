/*
Copyright 2024 The gomcache AUTHORS

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

// Package gomcache provides a client for the Memcached cache server using TCP and UDP.
package gomcache

import (
	"hash/crc32"
	"net"
	"strings"
	"sync"
)

// ServerSelector represents an interface for selecting servers.
type ServerSelector interface {
	// Select returns the server address that a given item should be sent to.
	Select(key string) (net.Addr, error)
	Each(func(net.Addr) error) error
}

// NewFromSelector returns a new Client using the provided ServerSelector and UDP mode.
func NewFromSelector(ss ServerSelector, useUDP bool) (*Client, error) {
	return &Client{
		selector: ss,
		UseUDP:   useUDP,
		Timeout:  DefaultTimeout,
	}, nil
}

// ServerList manages a list of servers.
type ServerList struct {
	mu    sync.RWMutex
	addrs []net.Addr
}

// staticAddr caches the Network() and String() values from any net.Addr.
type staticAddr struct {
	ntw, str string
}

func newStaticAddr(a net.Addr) *staticAddr {
	return &staticAddr{
		ntw: a.Network(),
		str: a.String(),
	}
}

func (s *staticAddr) Network() string { return s.ntw }
func (s *staticAddr) String() string  { return s.str }

// SetServers sets the list of servers.
// This method resolves server addresses and is safe for concurrent use.
func (ss *ServerList) SetServers(servers ...string) error {
	naddr := make([]net.Addr, len(servers))
	for i, server := range servers {
		var addr net.Addr
		var err error

		if strings.Contains(server, "/") {
			// Handle Unix domain sockets
			addr, err = net.ResolveUnixAddr("unix", server)
		} else if strings.Contains(server, ":") {
			// Handle TCP and UDP addresses
			// Try UDP first
			addr, err = net.ResolveUDPAddr("udp", server)
			if err != nil {
				// If UDP fails, try TCP
				addr, err = net.ResolveTCPAddr("tcp", server)
			}
		} else {
			// Default to TCP if no protocol is specified and address does not contain `/` or `:`
			addr, err = net.ResolveTCPAddr("tcp", server)
		}

		if err != nil {
			return err
		}
		naddr[i] = newStaticAddr(addr)
	}

	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.addrs = naddr
	return nil
}

// Each iterates over each server calling the given function
func (ss *ServerList) Each(f func(net.Addr) error) error {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	for _, a := range ss.addrs {
		if err := f(a); nil != err {
			return err
		}
	}
	return nil
}

// keyBufPool returns []byte buffers for use by PickServer's call to
// crc32.ChecksumIEEE to avoid allocations. (but doesn't avoid the
// copies, which at least are bounded in size and small)
var keyBufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 256)
		return &b
	},
}

// Select selects a server from the list using a consistent hashing strategy.
func (sl *ServerList) Select(key string) (net.Addr, error) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	if len(sl.addrs) == 0 {
		return nil, ErrNoServers
	}

	if len(sl.addrs) == 1 {
		return sl.addrs[0], nil
	}

	bufp := keyBufPool.Get().(*[]byte)
	n := copy(*bufp, []byte(key))

	// Use consistent hashing to select a server
	hash := crc32.ChecksumIEEE((*bufp)[:n])
	keyBufPool.Put(bufp)

	index := int(hash) % len(sl.addrs)

	return sl.addrs[index], nil
}
