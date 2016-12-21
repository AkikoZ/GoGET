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
	err = urlComponents.ResolveDNS()
	if err != nil {
		promptError("Failed to resolve DNS", err)
	}
	if urlComponents.IPv4 != nil {
		fmt.Fprintf(os.Stderr, "[INFO] Resolved IPv4 address: %s\n", urlComponents.IPv4)
	}
	if urlComponents.IPv6 != nil {
		fmt.Fprintf(os.Stderr, "[INFO] Resolved IPv6 address: %s\n", urlComponents.IPv6)
	}

	/* make HTTP request */
	response, err := urlComponents.RequestHTTP("")
	if err != nil {
		promptError("Failed to get HTTP response", err)
	}
	fmt.Print(response.Body)
}

func promptError(message string, err error) {
	fmt.Fprintf(os.Stderr, "[ERROR] %s: %s\n", message, err)
	os.Exit(0)
}
