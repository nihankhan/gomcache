# gomcache

`gomcache` is a Go library for interacting with Memcached servers using both TCP and UDP protocols. It provides a robust and simple client for performing standard Memcached operations such as setting, getting, deleting cache items, and checking server availability with high-level concurrency support.

## Features

- **TCP and UDP Support**: Choose between TCP or UDP protocols for communication with Memcached servers.
- **Simple and Intuitive API**: Easy-to-use methods for setting, getting, deleting cache items, and checking server availability with the Ping method.
- **High-Level Concurrency**: Thread-safe operations with built-in concurrency management for high-performance applications.
- **Error Handling**: Robust error handling for network and protocol errors, ensuring reliable interactions with Memcached servers.

## Installation

To use `gomcache` in your Go project, you need to install it using `go get`. Open your terminal and run:

```bash
go get github.com/nihankhan/gomcache
```

## Usage

Here's a quick guide on how to use the `gomcache` library in your Go application:

### Import the Package

```go
import "github.com/nihankhan/gomcache"
```

### Create a New Client

Create a new `Client` instance with the addresses of your Memcached servers:

```go
client, err := gomcache.NewClient([]string{"localhost:11211"}, true) // If false disable UDP or true Enable UDP
if err != nil {
    log.Fatalf("failed to create client: %v", err)
}
```

### Set an Item

Use the `Set` method to add or update an item in the cache:

```go
item := &gomcache.Item{
    Key:   "foo",
    Value: []byte("bar"),
    Flags: 0,
    Expiration: 3600, // in seconds
}

err := client.Set(item)
if err != nil {
    log.Fatalf("failed to set item: %v", err)
}
```

### Get an Item

Use the `Get` method to retrieve an item from the cache:

```go
item, err := client.Get("foo")
if err != nil {
    log.Fatalf("failed to get item: %v", err)
}
fmt.Printf("Value: %s\n", item.Value)
```

### Delete an Item

Use the `Delete` method to remove an item from the cache:

```go
err := client.Delete("foo")
if err != nil {
    log.Fatalf("failed to delete item: %v", err)
}
```

### Ping the Server

Use the `Ping` method to check if the server is responsive:

```go
err := client.Ping("version\r\n")
if err != nil {
    log.Fatalf("server is not responding: %v", err)
} else {
    fmt.Println("Server is responsive")
}
```

## Testing

To run tests for `gomcache`, use the `go test` command:

```bash
go test -v ./...
```

## Contribution

Contributions are welcome! If you have any ideas, improvements, or bug fixes, please open an issue or submit a pull request on the [GitHub repository](https://github.com/nihankhan/gomcache).

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## Contact

For any questions or feedback, please contact [Nihan Khan](mailto:nihan.khan@outlook.com).

---

Enjoy using `gomcache` and happy coding!
