package labnet

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const HTTP_TIMEOUT = time.Duration(5) * time.Second
const DOWNLOAD_THRESHOLD = 10000000 // 10 MB
const TOTAL_THREAD = 4

type Header map[string]string

type URLComponents struct {
	Protocol   string
	DomainName string
	Port       string
	URI        string
	IPv4       net.IP
	IPv6       net.IP
}

type HTTPRequest struct {
	Method     string
	DomainName string
	Port       string
	URI        string
	Header     Header
}

type HTTPResponse struct {
	Line   string
	Code   int
	Header Header
	Body   string
}

func ParseURL(rawInput string) (urlComponents *URLComponents, err error) {
	urlReg := regexp.MustCompile(`(\w+)://([^/:]+)(:\d*)?([^ ]*)`)
	if components := urlReg.FindStringSubmatch(rawInput); components != nil {
		urlComponents = &URLComponents{Protocol: components[1], DomainName: components[2], URI: components[4]}
		// check protocol
		if urlComponents.Protocol != "http" && urlComponents.Protocol != "https" {
			err = fmt.Errorf("protocol: %s not implemented", urlComponents.Protocol)
			return
		}
		// check port
		if len(components[3]) != 0 {
			urlComponents.Port = components[3][1:]
		} else if urlComponents.Protocol == "https" {
			urlComponents.Port = "443"
		} else {
			urlComponents.Port = "80"
		}
		// check URI
		if len(urlComponents.URI) == 0 {
			urlComponents.URI = "/"
		}
	} else {
		err = fmt.Errorf("invalid URL: %s", rawInput)
	}
	return
}

func (urlComponents *URLComponents) RequestHTTP(cookie string) (err error) {
	request := urlComponents.makeHTTPRequest("HEAD", cookie)
	var response *HTTPResponse
	// try IPv6 first
	if urlComponents.IPv6 != nil {
		response, err = request.send(urlComponents.IPv6, urlComponents.Protocol == "https")
		if err == nil {
			length := response.multiThreadDownloadLength()
			if length >= DOWNLOAD_THRESHOLD {
				// try multi-thread GET
				err = urlComponents.multiThreadDownload("ipv6", length, cookie)
				if err == nil {
					return
				}
				fmt.Fprintf(os.Stderr, "[WARNING] Multi-thread downloading failed: %s\n", err)
			}
		}
		// normal GET
		request = urlComponents.makeHTTPRequest("GET", cookie)
		response, err = request.send(urlComponents.IPv6, urlComponents.Protocol == "https")
		if err == nil {
			fmt.Print(response.Body)
			return
		}
		fmt.Fprintf(os.Stderr, "[WARNING] Failed to get HTTP response from the IPv6 address: %s\n", err)
	}
	// if failed, fall back to IPv4
	if urlComponents.IPv4 != nil {
		response, err = request.send(urlComponents.IPv4, urlComponents.Protocol == "https")
		if err == nil {
			length := response.multiThreadDownloadLength()
			if length >= DOWNLOAD_THRESHOLD {
				// try multi-thread GET
				err = urlComponents.multiThreadDownload("ipv4", length, cookie)
				if err == nil {
					return
				}
				fmt.Fprintf(os.Stderr, "[WARNING] Multi-thread downloading failed: %s\n", err)
			}
		}
		// normal GET
		request = urlComponents.makeHTTPRequest("GET", cookie)
		response, err = request.send(urlComponents.IPv4, urlComponents.Protocol == "https")
		if err == nil {
			fmt.Print(response.Body)
			return
		}
		fmt.Fprintf(os.Stderr, "[WARNING] Failed to get HTTP response from the IPv4 address: %s\n", err)
	}
	// both failed
	return
}

func (urlComponents *URLComponents) makeHTTPRequest(method string, cookie string) (request *HTTPRequest) {
	request = &HTTPRequest{Method: method, DomainName: urlComponents.DomainName, Port: urlComponents.Port, URI: urlComponents.URI, Header: make(Header)}
	request.SetDefaultHeader()
	if cookie != "" {
		request.Header["Cookie"] = cookie
	}
	return
}

func (response *HTTPResponse) multiThreadDownloadLength() (contentLength int) {
	contentLength = 0
	if accept, ok := response.Header["Accept-Ranges"]; ok && accept != "none" {
		if contentLengthString, ok := response.Header["Content-Length"]; ok {
			contentLength, _ = strconv.Atoi(contentLengthString)
		}
	}
	return
}

func (header Header) String() string {
	var s string
	for key, value := range header {
		s += key + ": " + value + "\r\n"
	}
	return s
}

func (request *HTTPRequest) SetDefaultHeader() {
	request.Header["Host"] = request.DomainName + ":" + request.Port
	request.Header["Accept"] = "*/*"
	request.Header["Accept-Encoding"] = "gzip"
	request.Header["Cache-Control"] = "no-cache"
	request.Header["Connection"] = "close"
}

func (request *HTTPRequest) String() string {
	line := request.Method + " " + request.URI + " HTTP/1.1\r\n"
	return line + request.Header.String() + "\r\n"
}

func (response *HTTPResponse) String() string {
	return response.Line + "\r\n" + response.Header.String() + "\r\n"
}

func (request *HTTPRequest) send(ip net.IP, isTLS bool) (response *HTTPResponse, err error) {
	/* dial TCP */
	var conn net.Conn
	if isTLS {
		dialer := &net.Dialer{Timeout: HTTP_TIMEOUT}
		conn, err = tls.DialWithDialer(dialer, "tcp", request.DomainName+":"+request.Port, nil)
	} else {
		conn, err = net.DialTimeout("tcp", getDialString(ip)+":"+request.Port, HTTP_TIMEOUT)
	}
	if err != nil {
		return
	}
	conn.SetDeadline(time.Now().Add(HTTP_TIMEOUT))

	/* send request */
	requestString := request.String()
	_, err = conn.Write([]byte(requestString))
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "[INFO] HTTP request sent:\n%s", requestString)

	/* get response */
	rawResponse, err := ioutil.ReadAll(conn)
	if err != nil {
		return
	}

	/* parse response */
	responseString := string(rawResponse)
	responseComponents := strings.Split(responseString, "\r\n")
	if len(responseComponents) < 3 {
		err = errors.New("bad response")
		return
	}
	// parse response line
	responseLine := responseComponents[0]
	responseLineComponents := strings.Split(responseLine, " ")
	if len(responseLineComponents) < 3 {
		err = errors.New("bad response line")
		return
	}
	response = &HTTPResponse{Line: responseLine}
	response.Code, err = strconv.Atoi(responseLineComponents[1])
	if err != nil {
		return
	}
	if len(responseComponents) > 3 {
		// parse response header
		response.Header = make(Header)
		cookie := ""
		for index, responseComponent := range responseComponents[1:] {
			if responseComponent == "" {
				responseComponents = responseComponents[index+2:]
				break
			}
			headerComponent := strings.Split(responseComponent, ": ")
			if len(headerComponent) > 2 {
				headerComponent[1] = strings.Join(headerComponent[1:], ": ")
			}
			if len(headerComponent) < 2 || headerComponent[0] == "" || headerComponent[1] == "" {
				err = errors.New("bad response header")
				return
			}
			// parse cookie (if exists)
			if headerComponent[0] == "Set-Cookie" {
				cookieComponents := strings.SplitAfter(headerComponent[1], "; ")
				if len(cookieComponents) == 0 {
					err = errors.New("bad response header")
					return
				}
				cookie += cookieComponents[0]
			}
			response.Header[headerComponent[0]] = headerComponent[1]
		}
		if cookie != "" {
			cookie = cookie[:len(cookie)-2]
		}
		if contentLength, ok := response.Header["Content-Length"]; ok {
			if match, _ := regexp.MatchString(`^\d+$`, contentLength); !match {
				err = errors.New("bad response header")
				return
			}
		}
		fmt.Fprintf(os.Stderr, "[INFO] Got HTTP response: %s\n", response)
		// follow HTTP redirect
		if response.Code == 301 || response.Code == 302 {
			if location, ok := response.Header["Location"]; ok {
				// parse redirecting URL
				urlComponents, urlErr := ParseURL(location)
				if urlErr != nil {
					err = urlErr
					return
				}
				// resolve DNS if needed
				if urlComponents.DomainName != request.DomainName {
					err = urlComponents.ResolveDNS()
					if err != nil {
						return
					}
					if urlComponents.IPv4 != nil {
						fmt.Fprintf(os.Stderr, "[INFO] Resolved redirecting IPv4 address: %s\n", urlComponents.IPv4)
					}
					if urlComponents.IPv6 != nil {
						fmt.Fprintf(os.Stderr, "[INFO] Resolved redirecting IPv6 address: %s\n", urlComponents.IPv6)
					}
				} else {
					if isIPv6 := ip.To4() == nil; isIPv6 {
						urlComponents.IPv6 = ip
					} else {
						urlComponents.IPv4 = ip
					}
				}
				// make new HTTP request
				err = urlComponents.RequestHTTP(cookie)
				return
			} else {
				err = errors.New("bad response header")
				return
			}
		}
		// parse response body
		if request.Method != "HEAD" {
			err = response.parseBody([]byte(strings.Join(responseComponents, "\r\n")))
			if err != nil {
				return
			}
		}
	}
	return
}

func (response *HTTPResponse) parseBody(raw []byte) (err error) {
	// check encoding para
	chunked := false
	gziped := false
	if transferEncoding, ok := response.Header["Transfer-Encoding"]; ok {
		chunked = transferEncoding == "chunked"
	}
	if contentEncoding, ok := response.Header["Content-Encoding"]; ok {
		gziped = contentEncoding == "gzip"
	}
	// decode if needed
	if !chunked && !gziped {
		response.Body = string(raw)
		return
	}
	var decoded []byte
	if chunked {
		for {
			// get chunk size
			sizeEnd := strings.Index(string(raw), "\r\n")
			if sizeEnd == -1 {
				err = errors.New("bad chunked body")
				return
			}
			rawSize := string(raw[:sizeEnd])
			size, atoiErr := strconv.ParseInt(rawSize, 16, 0)
			if atoiErr != nil {
				err = atoiErr
				return
			}
			if size < 0 {
				err = errors.New("bad chunked body")
				return
			}
			if size == 0 {
				break
			}
			// decode chunk
			raw = raw[sizeEnd+2:]
			decoded = append(decoded, raw[:size]...)
			raw = raw[size+2:]
		}
	}
	if gziped {
		if !chunked {
			decoded = raw
		}
		buffer := bytes.NewBuffer(decoded)
		r, gzipErr := gzip.NewReader(buffer)
		if gzipErr != nil {
			err = gzipErr
			return
		}
		defer r.Close()
		decoded, err = ioutil.ReadAll(r)
		if err != nil {
			return
		}
	}
	response.Body = string(decoded)
	return
}

func (urlComponents *URLComponents) multiThreadDownload(protocol string, length int, cookie string) (err error) {
	/* prepare temp file */
	tmpFile, err := os.Create("temp")
	if err != nil {
		return
	}

	/* get dst IP */
	var ip net.IP
	switch protocol {
	case "ipv4":
		ip = urlComponents.IPv4
	case "ipv6":
		ip = urlComponents.IPv6
	}

	/* start multi-thread downloading */
	successC := make(chan bool, TOTAL_THREAD)
	errC := make(chan error, TOTAL_THREAD)
	interrupt := false
	size := length / TOTAL_THREAD
	var (
		from int
		to   int
	)
	for i := 0; i < TOTAL_THREAD; i++ {
		// get range
		from = size * i
		if i == TOTAL_THREAD-1 {
			to = length - 1
		} else {
			to = from + size - 1
		}
		// make request
		request := urlComponents.makeHTTPRequest("GET", cookie)
		request.Header["Range"] = "bytes=" + strconv.Itoa(from) + "-" + strconv.Itoa(to)
		// download
		go request.download(ip, urlComponents.Protocol == "https", from, successC, errC, &interrupt)
	}

	/* get download result */
	successCount := 0
	for {
		success := <-successC
		if success {
			successCount++
			if successCount == TOTAL_THREAD {
				break
			}
		} else {
			err = <-errC
			return
		}
	}
	body, err := ioutil.ReadAll(tmpFile)
	if err != nil {
		return
	}
	fmt.Print(string(body))
	return
}

func (request *HTTPRequest) download(ip net.IP, isTLS bool, from int, successC chan bool, errC chan error, interrupt *bool) {
	/* dial TCP */
	var (
		conn    net.Conn
		connErr error
	)
	if isTLS {
		dialer := &net.Dialer{Timeout: HTTP_TIMEOUT}
		conn, connErr = tls.DialWithDialer(dialer, "tcp", request.DomainName+":"+request.Port, nil)
	} else {
		conn, connErr = net.DialTimeout("tcp", getDialString(ip)+":"+request.Port, HTTP_TIMEOUT)
	}
	if connErr != nil {
		errC <- connErr
		successC <- false
		return
	}
	conn.SetDeadline(time.Now().Add(HTTP_TIMEOUT))

	/* send request */
	requestString := request.String()
	_, err := conn.Write([]byte(requestString))
	if err != nil {
		errC <- err
		successC <- false
		return
	}

	/* get response */
	rawResponse, err := ioutil.ReadAll(conn)
	if err != nil {
		errC <- err
		successC <- false
		return
	}

	/* parse response */
	responseString := string(rawResponse)
	responseComponents := strings.Split(responseString, "\r\n")
	if len(responseComponents) < 3 {
		errC <- err
		successC <- false
		return
	}
	// parse response line
	responseLine := responseComponents[0]
	responseLineComponents := strings.Split(responseLine, " ")
	if len(responseLineComponents) < 3 {
		errC <- err
		successC <- false
		return
	}
	response := &HTTPResponse{Line: responseLine}
	response.Code, err = strconv.Atoi(responseLineComponents[1])
	if err != nil || response.Code != 206 {
		errC <- err
		successC <- false
		return
	}
	// ignore response header
	for index, responseComponent := range responseComponents[1:] {
		if responseComponent == "" {
			responseComponents = responseComponents[index+2:]
			break
		}
	}
	// parse response body
	response.Body = strings.Join(responseComponents, "\r\n")

	// write to temp file
	if !*interrupt {
		tmpFile, err := os.OpenFile("temp", os.O_WRONLY, os.ModeAppend)
		if err != nil {
			errC <- err
			successC <- false
			return
		}
		defer tmpFile.Close()
		_, err = tmpFile.WriteAt([]byte(response.Body), int64(from))
		if err != nil {
			errC <- err
			successC <- false
			return
		}
		successC <- true
	}
}
