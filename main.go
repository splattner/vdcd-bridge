package main

import (
	"fmt"
	"os"
	"strings"

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

	dryMode := p.Flag("", "dryMode", &argparse.Options{Required: false, Help: "only Discover, no adding"})

	mqttHost := p.String("", "mqtthost", &argparse.Options{Required: false, Help: "MQTT Host to connect to"})
	mqttUsername := p.String("", "mqttusername", &argparse.Options{Required: false, Help: "MQTT Username"})
	mqttPassword := p.String("", "mqttpassword", &argparse.Options{Required: false, Help: "MQTT Password"})

	homeassistantURL := p.String("", "homeassistant-url", &argparse.Options{Required: false, Help: "Home Assistant base URL (e.g. http://homeassistant.local:8123)"})
	homeassistantToken := p.String("", "homeassistant-token", &argparse.Options{Required: false, Help: "Home Assistant long-lived access token"})

	deconzHost := p.String("", "deconzhost", &argparse.Options{Required: false, Help: "Deconz Host IP"})
	deconzPort := p.Int("", "deconzport", &argparse.Options{Required: false, Help: "Deconz Port"})
	deconzWebSocketPort := p.Int("", "deconzwebsocketport", &argparse.Options{Required: false, Help: "Deconz Websocket Port"})
	deconzAPI := p.String("", "deconzapi", &argparse.Options{Required: false, Help: "Deconz API"})
	deconzEnableGroups := p.Flag("", "deconz-groupEnabled", &argparse.Options{Required: false, Help: "Deconz, enable adding Groups"})

	tasmotaDisabled := p.Flag("", "tasmotaDisabled", &argparse.Options{Required: false, Help: "disable Tasmota discovery"})
	shellyDisabled := p.Flag("", "shellyDisabled", &argparse.Options{Required: false, Help: "disable Shelly discovery"})
	deconzDisabled := p.Flag("", "deconzDisabled", &argparse.Options{Required: false, Help: "disable Deconz discovery"})
	zigbee2mqttDisabled := p.Flag("", "zigbee2mqtt", &argparse.Options{Required: false, Help: "disable zigbee2mqtt discovery"})
	wledDisabled := p.Flag("", "wledDisabled", &argparse.Options{Required: false, Help: "disable WLED discovery"})
	homeassistantDisabled := p.Flag("", "homeassistantDisabled", &argparse.Options{Required: false, Help: "disable Home Assistant discovery"})

	err := p.Parse(os.Args)
	if err != nil {
		// In case of error print error and print usage
		// This can also be done by passing -h or --help flags
		fmt.Print(p.Usage(err))
		os.Exit(1)
	}

	config, err := buildConfig(
		*host,
		*port,
		*modelName,
		*vendorName,
		*dryMode,
		*mqttHost,
		*mqttUsername,
		*mqttPassword,
		*homeassistantURL,
		*homeassistantToken,
		*deconzHost,
		*deconzPort,
		*deconzWebSocketPort,
		*deconzAPI,
		*deconzEnableGroups,
		*tasmotaDisabled,
		*shellyDisabled,
		*deconzDisabled,
		*zigbee2mqttDisabled,
		*wledDisabled,
		*homeassistantDisabled,
	)
	if err != nil {
		log.Error(err)
		os.Exit(1)
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

func buildConfig(
	host string,
	port int,
	modelName string,
	vendorName string,
	dryMode bool,
	mqttHost string,
	mqttUsername string,
	mqttPassword string,
	homeassistantURL string,
	homeassistantToken string,
	deconzHost string,
	deconzPort int,
	deconzWebSocketPort int,
	deconzAPI string,
	deconzEnableGroups bool,
	tasmotaDisabled bool,
	shellyDisabled bool,
	deconzDisabled bool,
	zigbee2mqttDisabled bool,
	wledDisabled bool,
	homeassistantDisabled bool,
) (*VcdcBridgeConfig, error) {
	config := new(VcdcBridgeConfig)
	config.host = strings.TrimSpace(host)
	config.port = port
	config.modelName = strings.TrimSpace(modelName)
	config.vendorName = strings.TrimSpace(vendorName)
	config.dryMode = dryMode

	config.mqttHost = strings.TrimSpace(mqttHost)
	config.mqttUsername = strings.TrimSpace(mqttUsername)
	config.mqttPassword = mqttPassword

	config.homeassistantURL = strings.TrimSpace(homeassistantURL)
	config.homeassistantToken = strings.TrimSpace(homeassistantToken)

	config.deconzHost = strings.TrimSpace(deconzHost)
	config.deconzPort = deconzPort
	config.deconcWebSockerPort = deconzWebSocketPort
	config.deconzApi = strings.TrimSpace(deconzAPI)
	config.deconzEnableGroups = deconzEnableGroups

	config.tasmotaDisabled = tasmotaDisabled
	config.shellyDisabled = shellyDisabled
	config.deconzDisabled = deconzDisabled
	config.zigbee2mqttDisabled = zigbee2mqttDisabled
	config.wledDisabled = wledDisabled
	config.homeassistantDisabled = homeassistantDisabled

	if config.mqttHost != "" {
		config.mqttDiscoveryEnabled = true
	}

	if config.mqttHost == "" && (!config.tasmotaDisabled || !config.shellyDisabled || !config.zigbee2mqttDisabled) {
		return nil, fmt.Errorf("mqtt host is required when MQTT-based discovery is enabled")
	}

	if !config.deconzDisabled {
		if config.deconzHost == "" || config.deconzApi == "" || config.deconzPort == 0 {
			return nil, fmt.Errorf("deconz discovery requires --deconzhost, --deconzport, and --deconzapi")
		}
	}

	if !config.homeassistantDisabled {
		if config.homeassistantURL == "" || config.homeassistantToken == "" {
			return nil, fmt.Errorf("home assistant discovery requires --homeassistant-url and --homeassistant-token")
		}
	}

	return config, nil
}
