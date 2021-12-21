package main

import (
	"fmt"
	"os"

	"github.com/akamensky/argparse"
	log "github.com/sirupsen/logrus"
)

const envLogLevel = "LOG_LEVEL"
const defaultLogLevel = log.InfoLevel

func main() {

	logLevel := getLogLevel()
	log.SetLevel(logLevel)

	p := argparse.NewParser("vdcd", "Use Tasmota/Shelly as exernal device for a plan44.ch vdcd")

	host := p.String("H", "host", &argparse.Options{Required: true, Help: "vdcd Host to connect to"})
	port := p.Int("p", "port", &argparse.Options{Required: false, Help: "Port of your vdcd host", Default: 8999})

	modelName := p.String("", "modelname", &argparse.Options{Required: false, Help: "modelName to Announce", Default: "go-client"})
	vendorName := p.String("", "vendorName", &argparse.Options{Required: false, Help: "vendorName to Announce", Default: "go-client"})

	mqttHost := p.String("", "mqtthost", &argparse.Options{Required: false, Help: "MQTT Host to connect to"})
	mqttUsername := p.String("", "mqttusername", &argparse.Options{Required: false, Help: "MQTT Username"})
	mqttPassword := p.String("", "mqttpassword", &argparse.Options{Required: false, Help: "MQTT Password"})

	deconzHost := p.String("", "deconzhost", &argparse.Options{Required: false, Help: "Deconz Host IP"})
	deconzPort := p.Int("", "deconzport", &argparse.Options{Required: false, Help: "Deconz Port"})
	deconzWebSocketPort := p.Int("", "deconzwebsocketport", &argparse.Options{Required: false, Help: "Deconz Websocket Port"})
	deconzAPI := p.String("", "deconzapi", &argparse.Options{Required: false, Help: "Deconz API"})
	deconzEnableGroups := p.Flag("", "deconz-groupEnabled", &argparse.Options{Required: false, Help: "Deconz, enable adding Groups"})

	tasmotaDisabled := p.Flag("", "tasmotaDisabled", &argparse.Options{Required: false, Help: "disable Tasmota discovery"})
	shellyDisabled := p.Flag("", "shellyDisabled", &argparse.Options{Required: false, Help: "disable Shelly discovery"})
	deconzDisabled := p.Flag("", "deconzDisabled", &argparse.Options{Required: false, Help: "disable Deconz discovery"})

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
	config.modelName = *modelName
	config.vendorName = *vendorName

	config.mqttHost = *mqttHost
	config.mqttUsername = *mqttUsername
	config.mqttPassword = *mqttPassword

	config.deconzHost = *deconzHost
	config.deconzPort = *deconzPort
	config.deconcWebSockerPort = *deconzWebSocketPort
	config.deconzApi = *deconzAPI
	config.deconzEnableGroups = *deconzEnableGroups

	if config.mqttHost != "" {
		config.mqttDiscoveryEnabled = true
	}
	config.shellyDisabled = *shellyDisabled
	config.tasmotaDisabled = *tasmotaDisabled
	config.deconzDisabled = *deconzDisabled

	// Disable if config not complete
	if config.deconzHost == "" || config.deconzApi == "" || config.deconzPort == 0 {
		config.deconzDisabled = true
	}

	vcdcbrige := new(VcdcBridge)
	vcdcbrige.NewVcdcBrige(*config)

}

func getLogLevel() log.Level {
	levelString, exists := os.LookupEnv(envLogLevel)
	if !exists {
		return defaultLogLevel
	}

	level, err := log.ParseLevel(levelString)
	if err != nil {
		log.Errorf("error parsing %s: %v", envLogLevel, err)
		return defaultLogLevel
	}

	return level
}
