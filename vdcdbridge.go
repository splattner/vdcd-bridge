package main

import (
	"fmt"

	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/splattner/vdcd-bridge/pkg/discovery"
	"github.com/splattner/vdcd-bridge/pkg/vdcdapi"
)

type VcdcBridgeConfig struct {
	host string
	port int

	modelName  string
	vendorName string

	dryMode bool

	mqttHost     string
	mqttUsername string
	mqttPassword string

	deconzHost          string
	deconzPort          int
	deconcWebSockerPort int
	deconzApi           string
	deconzEnableGroups  bool

	mqttDiscoveryEnabled bool
	tasmotaDisabled      bool
	shellyDisabled       bool
	deconzDisabled       bool
	zigbee2mqttDisabled  bool
}

type VcdcBridge struct {
	vdcdClient *vdcdapi.Client
	mqttClient mqtt.Client
	wg         sync.WaitGroup

	config VcdcBridgeConfig
}

func (e *VcdcBridge) NewVcdcBrige(config VcdcBridgeConfig) {

	e.config = config

	e.vdcdClient = new(vdcdapi.Client)

	e.vdcdClient.NewCient(e.config.host, e.config.port, e.config.modelName, e.config.vendorName, e.config.dryMode)
	e.vdcdClient.Connect()
	defer e.vdcdClient.Close()

	// Configure MQTT Client if enabled
	if config.mqttDiscoveryEnabled {
		log.WithField("Host", config.mqttHost).Info("Create MQTT Client")

		mqttBroker := fmt.Sprintf("tcp://%s", config.mqttHost)
		opts := mqtt.NewClientOptions().AddBroker(mqttBroker).SetClientID("vdcd_client")
		opts.SetKeepAlive(60 * time.Second)
		opts.SetPingTimeout(1 * time.Second)
		opts.SetProtocolVersion(3)
		opts.SetOrderMatters(false)
		opts.SetUsername(config.mqttUsername)
		opts.SetPassword(config.mqttPassword)

		e.mqttClient = mqtt.NewClient(opts)
	}

	e.wg.Add(2)
	go e.startDiscovery()
	go e.loopVcdcClient()

	log.Debug("Waiting for Waitgroup")
	e.wg.Wait()
	log.Debug("Waitgroup finished")
}

func (e *VcdcBridge) startDiscovery() {
	log.Debugln("Start startDiscovery")

	if e.config.mqttDiscoveryEnabled {

		log.Info("MQTT connect")

		// Connect to MQTT Broker
		if token := e.mqttClient.Connect(); token.Wait() && token.Error() != nil {
			log.WithError(token.Error()).Error("MQTT connect failed")
		}

		// Tasmota Device Discovery
		if !e.config.tasmotaDisabled {
			tasmotaDiscovery := new(discovery.TasmotaDevice)
			tasmotaDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
		}

		// Shelly Device Discovery
		if !e.config.shellyDisabled {

			shellyDiscovery := new(discovery.ShellyDevice)
			shellyDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
		}

		// Zigbee2MQTT Discovery
		if !e.config.zigbee2mqttDisabled {

			zigbee2mqttDiscovery := new(discovery.Zigbee2MQTTDevice)
			zigbee2mqttDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)

		}
	}

	if !e.config.deconzDisabled {

		deconzDiscovery := new(discovery.DeconzDevice)
		deconzDiscovery.StartDiscovery(e.vdcdClient, e.config.deconzHost, e.config.deconzPort, e.config.deconcWebSockerPort, e.config.deconzApi, e.config.deconzEnableGroups)
	}

	log.Debug("Calling Waitgroup done for startDiscovery")
	e.wg.Done()

}

func (e *VcdcBridge) loopVcdcClient() {
	log.Debug("Start loopVcdcClient")
	e.vdcdClient.Listen()

	log.Debug("Calling Waitgroup done for loopVcdcClient")
	e.wg.Done()
}
