// go-out
//
//	egress busting using:
//		letmeoutofyour.net by @mubix
//		allports.exposed by @bhinfosecurity
//
//	2018 @leonjza

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

var version = "1.0"

var (
	servicePtr    *string
	startPortPtr  *int
	endPortPtr    *int
	concurrentPtr *int
	useHTTPSPtr   *bool
	throttlePtr   *bool
)

type service struct {
	url   string
	match string
}

var services = map[string]service{
	"letmeout": service{url: "go-out.letmeoutofyour.net", match: "w00tw00t"},
	"allports": service{url: "allports.exposed", match: "<p>Open Port</p>"},
}

// maxedWaitGroup is a type to control the maximum
// number of goroutines in a wait group
type maxedWaitGroup struct {
	current chan int
	wg      sync.WaitGroup
}

func (m *maxedWaitGroup) Add() {
	m.current <- 1
	m.wg.Add(1)
}

func (m *maxedWaitGroup) Done() {
	<-m.current
	m.wg.Done()
}

func (m *maxedWaitGroup) Wait() {
	m.wg.Wait()
}

// validService ensures that we got a valid service from the
// -service commandline flag.
func validService(s *string) bool {

	for b := range services {
		if b == *s {
			return true
		}
	}

	return false
}

// validPort checks that we got a valid port from one of the
// port commandline flags.
func validPort(p int) bool {

	if p > 0 && p <= 65535 {
		return true
	}

	return false
}

// testHTTPEgress tests if a specific port is allowed to connect
// to the internet via http by matching the specific services' matcher
func (service *service) testHTTPEgress(port int) {

	var scheme string
	if *useHTTPSPtr {
		scheme = "https://"
	} else {
		scheme = "http://"
	}

	url, err := url.Parse(scheme + service.url + ":" + strconv.Itoa(port))
	if err != nil {
		panic(err)
	}

	timeout := time.Duration(5 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url.String())
	if err != nil {
		// fmt.Printf("No connection on port %d\n", port)
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	if strings.Contains(string(body), service.match) {
		fmt.Printf("[!] Egress on port %d\n", port)
	}
}

func validateFlags() bool {

	// Flag Validation
	if !validService(servicePtr) {
		fmt.Printf("%s is an invalid service. Please choose 'letmeout' or 'allports'\n", *servicePtr)
		return false
	}

	if *useHTTPSPtr && *servicePtr != "letmeout" {
		fmt.Println("Only the 'letmeout' service supports HTTPS, disabling HTTPS checking.")
		*useHTTPSPtr = false
	}

	if !validPort(*startPortPtr) || !validPort(*endPortPtr) {
		fmt.Println("Either the start port or end port was invalid / out of range.")
		return false
	}

	if *endPortPtr < *startPortPtr {
		fmt.Println("End port should be larger than the start port.")
		return false
	}

	return true
}

func main() {

	servicePtr = flag.String("service", "letmeout", "Use 'letmeout' or 'allports' for this run.")
	startPortPtr = flag.Int("start", 1, "The start port to use.")
	endPortPtr = flag.Int("end", 65535, "The end port to use.")
	concurrentPtr = flag.Int("w", 5, "Number of concurrent workers to spawn.")
	useHTTPSPtr = flag.Bool("https", true, "Egress bust using HTTPs (letmeout only)")
	throttlePtr = flag.Bool("throttle", false, "Throttle request speed. (random for a max of 10sec)")
	flag.Parse()

	if !validateFlags() {
		return
	}

	fmt.Println("===== Configuration =====")
	fmt.Printf("Service:	%s\n", *servicePtr)
	fmt.Printf("Start Port:	%d\n", *startPortPtr)
	fmt.Printf("End Port:	%d\n", *endPortPtr)
	fmt.Printf("Workers:	%d\n", *concurrentPtr)
	fmt.Printf("HTTPS On:	%t\n", *useHTTPSPtr)
	fmt.Printf("Throttle:	%t\n", *throttlePtr)
	fmt.Printf("=========================\n\n")

	tester := services[*servicePtr]

	start := time.Now()
	mwg := maxedWaitGroup{
		current: make(chan int, *concurrentPtr),
		wg:      sync.WaitGroup{},
	}

	// Process the ports in the range we got
	for port := *startPortPtr; port <= *endPortPtr; port++ {

		mwg.Add()

		go func(p int) {

			defer mwg.Done()

			if *throttlePtr {
				time.Sleep(time.Second * time.Duration(rand.Intn(10)))
			}

			tester.testHTTPEgress(p)

		}(port)
	}

	// Wait for the work to complete
	mwg.Wait()

	fmt.Printf("Done in %s\n", time.Since(start))
}
