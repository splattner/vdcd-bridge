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
	host         string
	port         int
	mqttHost     string
	mqttUsername string
	mqttPassword string
	modelName    string
	vendorName   string

	mqttDiscoveryEnabled bool
	tasmotaDisabled      bool
	shellyDisabled       bool
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

	e.vdcdClient.NewCient(e.config.host, e.config.port, e.config.modelName, e.config.vendorName)
	e.vdcdClient.Connect()
	defer e.vdcdClient.Close()

	// Configure MQTT Client if enabled
	if config.mqttDiscoveryEnabled {
		log.Infof("Connecting to MQTT Host: %s\n", config.mqttHost)

		mqttBroker := fmt.Sprintf("tcp://%s", config.mqttHost)
		opts := mqtt.NewClientOptions().AddBroker(mqttBroker).SetClientID("vdcd_cient")
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
	e.wg.Wait()

}

func (e *VcdcBridge) startDiscovery() {

	if e.config.mqttDiscoveryEnabled {

		// Connect to MQTT Broker
		if token := e.mqttClient.Connect(); token.Wait() && token.Error() != nil {
			log.Error("MQTT connect failed: ", token.Error())
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
	}

	e.wg.Done()

}

func (e *VcdcBridge) loopVcdcClient() {
	e.vdcdClient.Listen()
	e.wg.Done()
}
