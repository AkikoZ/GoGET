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
		promptError("Failed to parse URL", err)
	}

	/* resolve DNS */
	err = urlComponents.ResolveDNS()
	if err != nil {
		promptError("Failed to resolve DNS", err)
	}

	/* make HTTP request */
	err = urlComponents.RequestHTTP("")
	if err != nil {
		promptError("Failed to get HTTP response", err)
	}
}

func promptError(message string, err error) {
	fmt.Fprintf(os.Stderr, "[ERROR] %s: %s\n", message, err)
	os.Exit(0)
}
