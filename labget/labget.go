package main

import (
	"Project/LabGET/src/labnet"
	"fmt"
	"os"
)

func main() {
	/* check args */
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "[INFO] Usage: %s URL\n", os.Args[0])
		os.Exit(0)
	}

	/* parse URL */
	urlComponents, err := labnet.ParseURL(os.Args[1])
	if err != nil {
		promptError("Cannot parse URL", err)
	}
	if urlComponents.Protocol != "http" && urlComponents.Protocol != "https" {
		err = fmt.Errorf("protocol: %s not implemented", urlComponents.Protocol)
		promptError("Protocol not implemented", err)
	}

	/* resolve DNS */
	ip, err := urlComponents.ResolveDNS()
	if err != nil {
		promptError("Failed to resolve DNS", err)
	}
	fmt.Fprintf(os.Stderr, "[INFO] Resolved IP address: %s\n", ip)

	/* send HTTP request */
	request := &labnet.HTTPRequest{DomainName: urlComponents.DomainName, Port: urlComponents.Port, URI: urlComponents.URI, Header: make(labnet.Header)}
	request.SetDefaultHeader()
	response, err := request.SendTo(ip, urlComponents.Protocol == "https")
	if err != nil {
		promptError("Failed to get HTTP response", err)
	}
	fmt.Print(response.Body)
}

func promptError(message string, err error) {
	fmt.Fprintf(os.Stderr, "[ERROR] %s: %s\n", message, err)
	os.Exit(0)
}
