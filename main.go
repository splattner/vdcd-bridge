package main

import (
	"fmt"
	"os"

	"github.com/akamensky/argparse"
)

func main() {

	p := argparse.NewParser("go-vdcd-api-client", "Go Client for plan44's vdcd")

	host := p.String("H", "host", &argparse.Options{Required: true, Help: "vdcd Host to connect to"})
	port := p.Int("p", "port", &argparse.Options{Required: false, Help: "Port of your vdcd host", Default: 8999})
	mqttHost := p.String("", "mqtthost", &argparse.Options{Required: false, Help: "MQTT Host to connect to"})
	modelName := p.String("", "modelname", &argparse.Options{Required: false, Help: "modelName to Announce", Default: "go-client"})
	vendorName := p.String("", "vendorName", &argparse.Options{Required: false, Help: "vendorName to Announce", Default: "go-client"})

	err := p.Parse(os.Args)
	if err != nil {
		// In case of error print error and print usage
		// This can also be done by passing -h or --help flags
		fmt.Print(p.Usage(err))
		os.Exit(1)
	}

	config := new(VcdcBridgeConfig)
	config.host = *host
	config.port = *port
	config.mqttHost = *mqttHost
	config.modelName = *modelName
	config.vendorName = *vendorName
	if config.mqttHost != "" {
		config.mqttDiscoveryEnabled = true
	}

	vcdcbrige := new(VcdcBridge)
	vcdcbrige.NewVcdcBrige(*config)

}
