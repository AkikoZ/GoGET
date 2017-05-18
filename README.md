# LabGET
A simple HTTP Getter with **self-implemented** URL parsing, DNS resolution and HTTP GET request and response parsing.

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
2. Modify the `import` in **labget.go** to match the absolute path of the `labnet` package in your `$GOPATH`
3. Run the following command: `go install LabGET/labget`
4. Get the executable **labget** in your `$GOPATH/bin`

## Usage
**labget** accept only one argument, the URL:
```shell
labget http://www.test.com
```
If there is no error, **labget** will print the received HTTP response body result to `stdout`. It will also print the resolved IP address and HTTP request/response header in `stderr`.
If anything goes wrong, **labget** will leave `stdout` empty and send error message to `stderr`.

## Libraries used for main functions
* Go's native TLS library
* Go's native gzip library

## References
* [RFC 1035](https://tools.ietf.org/html/rfc1035)
* [RFC 7230](https://tools.ietf.org/html/rfc7230)
