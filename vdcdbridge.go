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

	log.Debugln("Waiting for Waitgroup")
	e.wg.Wait()
	log.Debugln("Waitgroup finished")
}

func (e *VcdcBridge) startDiscovery() {
	log.Debugln("Start startDiscovery")

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

	if !e.config.deconzDisabled {

		deconzDiscovery := new(discovery.DeconzDevice)
		deconzDiscovery.StartDiscovery(e.vdcdClient, e.config.deconzHost, e.config.deconzPort, e.config.deconcWebSockerPort, e.config.deconzApi, e.config.deconzEnableGroups)
	}

	log.Debugln("Calling Waitgroup done for startDiscovery")
	e.wg.Done()

}

func (e *VcdcBridge) loopVcdcClient() {
	log.Debugln("Start loopVcdcClient")
	e.vdcdClient.Listen()

	log.Debugln("Calling Waitgroup done for loopVcdcClient")
	e.wg.Done()
}
