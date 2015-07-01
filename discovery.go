//
//  discovery.go
//
//  Created by Martino Facchin
//  Copyright (c) 2015 Arduino LLC
//
//  Permission is hereby granted, free of charge, to any person
//  obtaining a copy of this software and associated documentation
//  files (the "Software"), to deal in the Software without
//  restriction, including without limitation the rights to use,
//  copy, modify, merge, publish, distribute, sublicense, and/or sell
//  copies of the Software, and to permit persons to whom the
//  Software is furnished to do so, subject to the following
//  conditions:
//
//  The above copyright notice and this permission notice shall be
//  included in all copies or substantial portions of the Software.
//
//  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
//  EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES
//  OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
//  NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
//  HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
//  WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
//  FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
//  OTHER DEALINGS IN THE SOFTWARE.
//

package main

import (
	"github.com/oleksandr/bonjour"
	"log"
	"net/http"
	"strings"
	"time"
)

const timeoutConst = 2

// SavedNetworkPorts contains the ports which we know are already connected
var SavedNetworkPorts []OsSerialPort

// GetNetworkList returns a list of Network Ports
// The research of network ports is articulated in two phases. First we add new ports coming from
// the bonjour module, then we prune the boards who don't respond to a ping
func GetNetworkList() ([]OsSerialPort, error) {
	newPorts, err := getPorts()
	log.Println("newports", newPorts)
	if err != nil {
		return nil, err
	}

	tmp := SavedNetworkPorts

	for index, p1 := range tmp {
		for _, p2 := range newPorts {
			if p1.Name == p2.Name && p2.FriendlyName == p2.FriendlyName {
				copy(SavedNetworkPorts[index:], SavedNetworkPorts[index+1:])
				SavedNetworkPorts[len(SavedNetworkPorts)-1] = OsSerialPort{}
				SavedNetworkPorts = SavedNetworkPorts[:len(SavedNetworkPorts)-1]
			}
		}
	}

	SavedNetworkPorts, err = pruneUnreachablePorts(SavedNetworkPorts)
	if err != nil {
		return nil, err
	}

	SavedNetworkPorts = append(SavedNetworkPorts, newPorts...)

	return SavedNetworkPorts, nil
}

func pruneUnreachablePorts(ports []OsSerialPort) ([]OsSerialPort, error) {
	tmp := ports

	timeout := time.Duration(2 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}

	for index, port := range tmp {
		res, err := client.Head("http://" + port.Name)

		if err != nil || res.StatusCode != 200 {
			copy(ports[index:], ports[index+1:])
			ports[len(ports)-1] = OsSerialPort{}
			ports = ports[:len(ports)-1]
			log.Println("TIMEOUT?", err, res)
		}
	}
	return ports, nil
}

func getPorts() ([]OsSerialPort, error) {
	resolver, err := bonjour.NewResolver(nil)
	if err != nil {
		log.Println("Failed to initialize resolver:", err.Error())
		return nil, err
	}

	results := make(chan *bonjour.ServiceEntry)

	timeout := make(chan bool, 1)
	go func(exitCh chan<- bool) {
		time.Sleep(timeoutConst * time.Second)
		exitCh <- true
		close(results)
	}(resolver.Exit)

	arrPorts := []OsSerialPort{}
	go func(results chan *bonjour.ServiceEntry, exitCh chan<- bool) {
		for e := range results {
			log.Printf("%s %s %d %s", e.Instance, e.AddrIPv4, e.Port, e.Text)
			var boardInfosSlice []string
			for _, element := range e.Text {
				if strings.Contains(element, "board=yun") {
					boardInfosSlice = append(boardInfosSlice, "arduino:avr:yun")
				}
			}
			arrPorts = append(arrPorts, OsSerialPort{Name: e.AddrIPv4.String(), FriendlyName: e.Instance, NetworkPort: true, RelatedNames: boardInfosSlice})
		}
		timeout <- true
	}(results, resolver.Exit)

	err = resolver.Browse("_arduino._tcp", "", results)
	if err != nil {
		log.Println("Failed to browse:", err.Error())
		return nil, err
	}
	// wait for some kind of timeout and return arrPorts
	select {
	case <-timeout:
		return arrPorts, nil
	}
}
