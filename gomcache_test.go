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
	"fmt"
	"testing"
)

// MockServer simulates a Memcached server for testing purposes.
type MockServer struct {
	Addr string
}

// NewMockServer creates a new mock server.
func NewMockServer(addr string) *MockServer {
	return &MockServer{Addr: addr}
}

// Set simulates adding or updating an item in the mock server.
func (s *MockServer) Set(key string, value []byte) string {
	return "STORED"
}

// Get simulates retrieving an item from the mock server.
func (s *MockServer) Get(key string) (string, []byte) {
	if key == "existing_key" {
		return "VALUE", []byte("test_value")
	}
	return "END", nil
}

// Delete simulates removing an item from the mock server.
func (s *MockServer) Delete(key string) string {
	if key == "existing_key" {
		return "DELETED"
	}
	return "NOT_FOUND"
}

// TestSet tests the Set method.
func TestSet(t *testing.T) {
	mockServer := NewMockServer("localhost:11211")
	client, _ := NewClient([]string{mockServer.Addr}, false)

	item := &Item{
		Key:   "foo",
		Value: []byte("barrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrrr"),
	}

	err := client.Set(item)
	fmt.Println("error: ", err)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestGet tests the Get method with UDP.
func TestGet(t *testing.T) {
	mockServer := NewMockServer("localhost:11211")
	client, _ := NewClient([]string{mockServer.Addr}, true)

	// Simulate UDP response by using a real UDP connection or adapt the mockServer.
	item, err := client.Get("foo")
	fmt.Println("Item: ", item)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// if string(item.Value) != "test_value" {
	// 	t.Fatalf("expected value %s, got %s", "test_value", string(item.Value))
	// }

	_, err = client.Get("non_existing_key")
	if err == nil {
		t.Fatalf("expected an error, got nil")
	}
}

// TestDelete tests the Delete method.
func TestDelete(t *testing.T) {
	mockServer := NewMockServer("localhost:11211")
	client, _ := NewClient([]string{mockServer.Addr}, false)

	err := client.Delete("foo")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	err = client.Delete("non_existing_key")
	if err == nil {
		t.Fatalf("expected an error, got nil")
	}
}
