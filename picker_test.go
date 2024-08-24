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
	"net"
	"reflect"
	"strings"
	"testing"
)

func TestSetServers(t *testing.T) {
	serverList := &ServerList{}
	servers := []string{"localhost:11211", "localhost:11212"}

	err := serverList.SetServers(servers...)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(serverList.addrs) != len(servers) {
		t.Fatalf("expected %d servers, got %d", len(servers), len(serverList.addrs))
	}

	for i, server := range servers {
		expectedAddr := newStaticAddr(parseAddr(t, server))
		if !reflect.DeepEqual(serverList.addrs[i], expectedAddr) {
			t.Fatalf("expected server %v, got %v", expectedAddr, serverList.addrs[i])
		}
	}
}

func parseAddr(t *testing.T, server string) net.Addr {
	var addr net.Addr
	var err error

	if strings.Contains(server, "/") {
		addr, err = net.ResolveUnixAddr("unix", server)
	} else if strings.Contains(server, ":") {
		addr, err = net.ResolveUDPAddr("udp", server)
		if err != nil {
			addr, err = net.ResolveTCPAddr("tcp", server)
		}
	} else {
		addr, err = net.ResolveTCPAddr("tcp", server)
	}

	if err != nil {
		t.Fatalf("failed to resolve address %s: %v", server, err)
	}
	return newStaticAddr(addr)
}

func TestSelectServer(t *testing.T) {
	serverList := &ServerList{}
	servers := []string{"192.168.0.119:11211", "192.168.0.119:11212"}

	err := serverList.SetServers(servers...)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	key := "test_key"
	selectedServer, err := serverList.Select(key)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Check if the selected server is one of the servers in the list
	isValidServer := false
	for _, server := range servers {
		if selectedServer.String() == server {
			isValidServer = true
			break
		}
	}

	if !isValidServer {
		t.Fatalf("expected selected server to be one of %v, got %s", servers, selectedServer)
	}
}

func TestSelectNoServers(t *testing.T) {
	serverList := &ServerList{}

	_, err := serverList.Select("test_key")
	if err == nil {
		t.Fatalf("expected an error, got nil")
	}

	if err.Error() != "no servers available" {
		t.Fatalf("expected error 'no servers available', got %v", err)
	}
}

func TestSelectSingleServer(t *testing.T) {
	serverList := &ServerList{}
	servers := []string{"192.168.0.119:11211"}

	err := serverList.SetServers(servers...)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	selectedServer, err := serverList.Select("test_key")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if selectedServer.String() != servers[0] {
		t.Fatalf("expected server %s, got %s", servers[0], selectedServer)
	}
}

func TestEach(t *testing.T) {
	serverList := &ServerList{}
	servers := []string{"192.168.0.119:11211", "192.168.0.119:11212"}

	err := serverList.SetServers(servers...)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	count := 0
	err = serverList.Each(func(addr net.Addr) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if count != len(servers) {
		t.Fatalf("expected %d servers, got %d", len(servers), count)
	}
}

func TestThreadSafety(t *testing.T) {
	serverList := &ServerList{}
	servers := []string{"localhost:11211", "localhost:11212"}

	err := serverList.SetServers(servers...)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	const goroutines = 100
	done := make(chan bool)

	for i := 0; i < goroutines; i++ {
		go func() {
			_, err := serverList.Select("test_key")
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			done <- true
		}()
	}

	for i := 0; i < goroutines; i++ {
		<-done
	}
}

func TestSetServersWithDifferentProtocols(t *testing.T) {
	serverList := &ServerList{}
	servers := []string{"192.168.0.119:11211", "/tmp/memcached.sock"}

	err := serverList.SetServers(servers...)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for i, server := range servers {
		expectedAddr := parseAddr(t, server)
		addr := serverList.addrs[i]
		if !reflect.DeepEqual(addr, newStaticAddr(expectedAddr)) {
			t.Fatalf("expected address %v for server %s, got %v", expectedAddr, server, addr)
		}
	}
}
