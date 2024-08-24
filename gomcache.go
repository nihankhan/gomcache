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
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

var (
	ErrCacheMiss    = errors.New("memcache: cache miss")
	ErrNotStored    = errors.New("memcache: item not stored")
	ErrServerError  = errors.New("memcache: server error")
	ErrNoStats      = errors.New("memcache: no statistics available")
	ErrCASConflict  = errors.New("memcache: compare-and-swap conflict")
	ErrMalformedKey = errors.New("malformed: key is too long or contains invalid characters")
	ErrNoServers    = errors.New("memcache: no servers configured or available")
)

const (
	// DefaultTimeout is the default socket read/write timeout.
	DefaultTimeout = 500 * time.Millisecond

	// DefaultMaxIdleConns is the default maximum number of idle connections
	// kept for any single address.
	DefaultMaxIdleConns = 2
)

var (
	crlf           = []byte("\r\n")
	resultStored   = []byte("STORED\r\n")
	resultNotFound = []byte("NOT_FOUND\r\n")
	resultDeleted  = []byte("DELETED\r\n")
	resultEnd      = []byte("END\r\n")
	versionPrefix  = []byte("VERSION")
)

// Client represents a Memcached client.
type Client struct {
	selector ServerSelector
	UseUDP   bool

	// Timeout specifies the socket read/write timeout. If zero, DefaultTimeout is used.
	Timeout time.Duration
	mu      sync.Mutex
}

// Item represents a Memcached item.
type Item struct {
	Key        string
	Value      []byte
	Flags      uint32
	Expiration int32
}

// NewClient creates a new Client with the specified servers and UDP mode.
func NewClient(servers []string, useUDP bool) (*Client, error) {
	ss := new(ServerList)
	if err := ss.SetServers(servers...); err != nil {
		return nil, ErrNoServers
	}

	return NewFromSelector(ss, useUDP)
}

// SelectServer selects a server using the selector.
func (c *Client) SelectServer(key string) (string, error) {
	addr, err := c.selector.Select(key)
	if err != nil {
		return "", err
	}

	return addr.String(), nil
}

// connect establishes a TCP connection to the selected Memcached server.
func (c *Client) connect(key string) (net.Conn, error) {
	addr, err := c.SelectServer(key)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("tcp", addr, c.Timeout)
	if err != nil {
		return nil, err
	}

	err = conn.SetDeadline(time.Now().Add(c.Timeout))
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// connectUDP establishes a UDP connection to the selected Memcached server.
func (c *Client) connectUDP(key string) (*net.UDPConn, error) {
	addr, err := c.SelectServer(key)
	if err != nil {
		return nil, err
	}
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}

	// Set the read and write deadline based on the timeout
	err = conn.SetDeadline(time.Now().Add(c.Timeout))
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// Set adds or updates an item in the Memcached server using TCP.
func (c *Client) Set(item *Item) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Establish a TCP connection to the server
	conn, err := c.connect(item.Key)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Create and send the 'set' command
	req := fmt.Sprintf("set %s %d %d %d\r\n%s\r\n", item.Key, item.Flags, item.Expiration, len(item.Value), string(item.Value))
	_, err = conn.Write([]byte(req))
	if err != nil {
		return err
	}

	// Read the response
	resp, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return ErrServerError
	}

	// Compare the response with predefined byte slices
	switch {
	case bytes.Equal(resp, resultStored):
		return nil
	case bytes.Equal(resp, resultNotFound):
		return ErrCacheMiss
	default:
		return fmt.Errorf("unexpected response: %s", resp)
	}
}

// Get retrieves an item from the Memcached server using UDP.
func (c *Client) Get(key string) (*Item, error) {
	if !c.UseUDP {
		return nil, fmt.Errorf("UDP mode is not enabled")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := c.connectUDP(key)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Create the request frame header
	frameHeader := make([]byte, 8)
	binary.BigEndian.PutUint16(frameHeader[0:2], 0) // Request ID
	binary.BigEndian.PutUint16(frameHeader[2:4], 0) // Sequence number
	binary.BigEndian.PutUint16(frameHeader[4:6], 1) // Total number of datagrams
	binary.BigEndian.PutUint16(frameHeader[6:8], 0) // Reserved

	// Prepare the Get command
	getCommand := append(frameHeader, []byte("get "+key)...)

	// Send the Get command
	_, err = conn.Write(append(getCommand, crlf...))
	if err != nil {
		return nil, fmt.Errorf("error writing to UDP: %v", err)
	}

	// Read the response
	buffer := make([]byte, 90000) // Buffer size for UDP
	var responseBuffer bytes.Buffer
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			return nil, fmt.Errorf("error reading from UDP: %v", err)
		}

		// Append the data to the response buffer
		responseBuffer.Write(buffer[8:n])

		// Check for the end of the response
		if bytes.Contains(responseBuffer.Bytes(), resultEnd) {
			break
		}
	}

	// Parse the response
	rawResponse := responseBuffer.Bytes()
	if bytes.HasPrefix(rawResponse, []byte("VALUE")) {
		lines := bytes.Split(rawResponse, crlf)
		if len(lines) >= 2 {
			value := lines[1] // Extract the value part
			return &Item{
				Key:   key,
				Value: value,
			}, nil
		}
	} else if bytes.HasPrefix(rawResponse, resultNotFound) {
		return nil, ErrCacheMiss
	}

	return nil, fmt.Errorf("unexpected response: %s", rawResponse)
}

// Delete removes an item from the Memcached server using TCP.
func (c *Client) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := c.connect(key)
	if err != nil {
		return err
	}
	defer conn.Close()

	req := fmt.Sprintf("delete %s\r\n", key)
	_, err = conn.Write([]byte(req))
	if err != nil {
		return err
	}

	resp, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return ErrServerError
	}

	// Compare the response with predefined byte slices
	switch {
	case bytes.Equal(resp, resultDeleted):
		return nil
	case bytes.Equal(resp, resultNotFound):
		return fmt.Errorf("item not found")
	default:
		return fmt.Errorf("unexpected response: %s", resp)
	}
}

// Ping checks if the server is responsive by sending a "version" command.
func (c *Client) Ping(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, err := c.connect(key)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Send the "version" command
	_, err = conn.Write(append(versionPrefix, crlf...))
	if err != nil {
		return err
	}

	// Read the response
	resp, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return ErrServerError
	}

	// Check if the response starts with "VERSION"
	if bytes.HasPrefix(resp, versionPrefix) {
		return nil
	}

	return fmt.Errorf("unexpected response: %s", resp)
}
