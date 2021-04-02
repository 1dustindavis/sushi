package main

import (
	"fmt"
	"net"
	"time"
)

func main() {

	// get the sushi config

	// check if chef-client is installed

	// check if client.rb is configured

	// check if the chef server is available
	if serverAvailable("chef.example.com:443") {
		// if chef server is available, run in server mode

		// shell out and run chef-client with client-server.rb

	} else {
		// if chef server is NOT available, run in solo mode

		// check if the sushi repo is available
		if serverAvailable("sushi.example.com:443") {
			// if sushi repo is available, check the current version and save the expiration

			// compare local repo version, if needed download latest version

			// shell out and run chef-client with client-solo.rb
		} else {
			// if sushi repo is NOT available, check if our local repo has expired
			if localRepoExpired() {
				// if the local repo has expired, exit without running chef

			} else {
				// shell out and run chef-client with client-solo.rb
			}
		}
	}
}

func serverAvailable(serverURL string) bool {
	// needs to add support for file:/// urls
	timeout := 1 * time.Second
	conn, err := net.DialTimeout("tcp", serverURL, timeout)
	if err != nil {
		fmt.Println(conn)
		fmt.Println("Site unreachable, error: ", err)
		return false
	}
	return true
}

func localRepoExpired() bool {
	return false
}
