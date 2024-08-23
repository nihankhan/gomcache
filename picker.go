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
	"errors"
	"hash/crc32"
	"net"
	"strings"
	"sync"
)

// ServerSelector represents an interface for selecting servers.
type ServerSelector interface {
	// Select returns the server address that a given item should be sent to.
	Select(key string) (string, error)
	Each(func(string) error) error
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
	mu      sync.RWMutex
	servers []string
	addrs   []net.Addr
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
func (sl *ServerList) SetServers(servers ...string) error {
	addrs := make([]net.Addr, len(servers))
	for i, server := range servers {
		var addr net.Addr
		var err error
		if strings.Contains(server, "/") {
			addr, err = net.ResolveUnixAddr("unix", server)
		} else {
			addr, err = net.ResolveTCPAddr("tcp", server)
		}
		if err != nil {
			return err
		}
		addrs[i] = newStaticAddr(addr)
	}

	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.servers = servers
	sl.addrs = addrs
	return nil
}

// Each iterates over each server and calls the given function.
func (sl *ServerList) Each(f func(string) error) error {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	for _, addr := range sl.addrs {
		if err := f(addr.String()); err != nil {
			return err
		}
	}
	return nil
}

// Select selects a server from the list using a consistent hashing strategy.
func (sl *ServerList) Select(key string) (string, error) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	if len(sl.addrs) == 0 {
		return "", errors.New("no servers available")
	}
	if len(sl.addrs) == 1 {
		return sl.addrs[0].String(), nil
	}

	// Use consistent hashing to select a server
	hash := crc32.ChecksumIEEE([]byte(key))
	index := int(hash) % len(sl.addrs)
	return sl.addrs[index].String(), nil
}
