# GoGET

A Go implementation of an HTTP Getter with **fully self-implemented** URL parsing, DNS resolution and HTTP GET request and response parsing **using only transport layer APIs**.

## Functions

* HTTP/HTTPS scheme with default or custom port
* Concurrent DNS resolution for both IPv4 and IPv6 addresses
* HTTP 1.1 GET request and response parsing
* HTTP redirect following with cookie handled
* Chunked HTTP response body supported
* HTTP response body with gzip encoding supported
* Multi-thread downloading large response bodies

## Installing

1. Clone the git repository to your `$GOPATH`
1. Modify the `import` in **goget.go** to match the absolute path of the `gonet` package in your `$GOPATH`
1. Run the following command: `go install GoGET/goget`
1. Get the executable **goget** in your `$GOPATH/bin`

## Usage

**goget** accept only one argument, the URL:

```shell
goget http://www.test.com
```

If there is no error, **goget** will print the received HTTP response body result to `stdout`. It will also print the resolved IP address and HTTP request/response header in `stderr`.
If anything goes wrong, **goget** will leave `stdout` empty and send error message to `stderr`.

## Libraries used for main functions

* Go's native TLS library
* Go's native gzip library

## References

* [RFC 1035](https://tools.ietf.org/html/rfc1035)
* [RFC 7230](https://tools.ietf.org/html/rfc7230)
