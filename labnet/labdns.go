package labnet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
)

const DNS_TIMEOUT = time.Duration(5) * time.Second
const DNS_RESOLVER = "202.120.224.26:53"

type dnsHeader struct {
	id      uint16
	flag    uint16
	qdCount uint16
	anCount uint16
	nsCount uint16
	arCount uint16
}

type dnsQuestionProp struct {
	qType  uint16
	qClass uint16
}

type dnsResolutionResult struct {
	ip  net.IP
	err error
}

func (urlComponents *URLComponents) ResolveDNS() (err error) {
	ipv4Result := &dnsResolutionResult{}
	ipv6Result := &dnsResolutionResult{}
	ipv4Complete := make(chan int)
	ipv6Complete := make(chan int)
	errCnt := 0
	go urlComponents.resolveDNS("ipv4", ipv4Result, ipv4Complete)
	go urlComponents.resolveDNS("ipv6", ipv6Result, ipv6Complete)
	for {
		select {
		case <-ipv4Complete:
			if ipv4Result.err == nil {
				urlComponents.IPv4 = ipv4Result.ip
				return
			} else {
				errCnt++
				if errCnt == 2 {
					err = fmt.Errorf("ipv4 error: %s, ipv6 error: %s", ipv4Result.err, ipv6Result.err)
					return
				}
			}
		case <-ipv6Complete:
			if ipv6Result.err == nil {
				urlComponents.IPv6 = ipv6Result.ip
				return
			} else {
				errCnt++
				if errCnt == 2 {
					err = fmt.Errorf("ipv4 error: %s, ipv6 error: %s", ipv4Result.err, ipv6Result.err)
					return
				}
			}
		}
	}
}

func (urlComponents *URLComponents) resolveDNS(protocol string, result *dnsResolutionResult, complete chan int) {
	/* set query type */
	var qType uint16
	switch protocol {
	case "ipv4":
		qType = 1
	case "ipv6":
		qType = 28
	}

	/* generate id */
	queryID := uint16(rand.Int())

	/* make DNS query */
	queryHeader := dnsHeader{id: queryID, qdCount: 1, anCount: 0, nsCount: 0, arCount: 0}
	queryHeader.setFlag(0, 0, 0, 0, 1, 0, 0)
	questionProp := dnsQuestionProp{qType: qType, qClass: 1}

	/* dial UDP */
	conn, err := net.DialTimeout("udp", DNS_RESOLVER, DNS_TIMEOUT)
	if err != nil {
		result.err = err
		complete <- 0
		return
	}
	conn.SetDeadline(time.Now().Add(DNS_TIMEOUT))

	/* send DNS query */
	buffer := bytes.Buffer{}
	binary.Write(&buffer, binary.BigEndian, queryHeader)
	binary.Write(&buffer, binary.BigEndian, parseDomainName(urlComponents.DomainName))
	binary.Write(&buffer, binary.BigEndian, questionProp)
	querySize := len(buffer.Bytes())
	_, err = conn.Write(buffer.Bytes())
	if err != nil {
		result.err = err
		complete <- 0
		return
	}

	/* parse DNS response */
	rawResponse := make([]byte, 1024)
	_, err = conn.Read(rawResponse)
	if err != nil {
		result.err = err
		complete <- 0
		return
	}
	// check id
	var responseID uint16
	binary.Read(bytes.NewBuffer(rawResponse[0:2]), binary.BigEndian, &responseID)
	if responseID != queryID {
		result.err = errors.New("invalid DNS response from server")
		complete <- 0
		return
	}
	// check response code
	var responseFlag uint16
	binary.Read(bytes.NewBuffer(rawResponse[2:4]), binary.BigEndian, &responseFlag)
	if responseFlag<<12 != 0 {
		result.err = errors.New("server cannot resolve DNS")
		complete <- 0
		return
	}
	// parse raw answer
	rawAnswer := rawResponse[querySize:]
	for {
		// skip name
		for {
			indicator := uint8(rawAnswer[0])
			// check if next is a pointer
			if indicator>>6 == 3 {
				rawAnswer = rawAnswer[2:]
				break
			} else {
				// check if name ends
				if indicator == 0 {
					rawAnswer = rawAnswer[1:]
					break
				} else {
					rawAnswer = rawAnswer[1+indicator:]
				}
			}
		}
		// get answer type and data length
		var (
			aType      uint16
			dataLength uint16
		)
		binary.Read(bytes.NewBuffer(rawAnswer[0:2]), binary.BigEndian, &aType)
		binary.Read(bytes.NewBuffer(rawAnswer[8:10]), binary.BigEndian, &dataLength)
		// check type
		if aType == qType {
			// get IP
			if qType == 1 {
				result.ip = net.IPv4(rawAnswer[10], rawAnswer[11], rawAnswer[12], rawAnswer[13])
				complete <- 0
			} else {
				result.ip = make(net.IP, 16)
				result.ip = net.IP(rawAnswer[10:26])
				complete <- 0
			}
			return
		}
		// get next answer (if exists)
		if len(rawAnswer) < int(22+dataLength) {
			result.err = fmt.Errorf("the host does not have an %s address", protocol)
			complete <- 0
			return
		}
		rawAnswer = rawAnswer[10+dataLength:]
	}
}

func (header *dnsHeader) setFlag(QR uint16, OpCode uint16, AA uint16, TC uint16, RD uint16, RA uint16, RCode uint16) {
	header.flag = QR<<15 + OpCode<<11 + AA<<10 + TC<<9 + RD<<8 + RA<<7 + RCode
}

func parseDomainName(domain string) []byte {
	segments := strings.Split(domain, ".")
	buffer := bytes.Buffer{}
	for _, segment := range segments {
		binary.Write(&buffer, binary.BigEndian, byte(len(segment)))
		binary.Write(&buffer, binary.BigEndian, []byte(segment))
	}
	binary.Write(&buffer, binary.BigEndian, byte(0x00))
	return buffer.Bytes()
}
