package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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

	homeassistantURL   string
	homeassistantToken string

	deconzHost          string
	deconzPort          int
	deconcWebSockerPort int
	deconzApi           string
	deconzEnableGroups  bool

	mqttDiscoveryEnabled  bool
	tasmotaDisabled       bool
	shellyDisabled        bool
	deconzDisabled        bool
	zigbee2mqttDisabled   bool
	wledDisabled          bool
	homeassistantDisabled bool
}

type VcdcBridge struct {
	vdcdClient *vdcdapi.Client
	mqttClient mqtt.Client
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc

	config VcdcBridgeConfig
}

const discoveryInterval = 5 * time.Minute

func (e *VcdcBridge) NewVcdcBrige(config VcdcBridgeConfig) {

	e.config = config

	e.vdcdClient = new(vdcdapi.Client)

	e.vdcdClient.NewCient(e.config.host, e.config.port, e.config.modelName, e.config.vendorName, e.config.dryMode)
	e.vdcdClient.Connect()
	//defer e.vdcdClient.Close()

	e.ctx, e.cancel = context.WithCancel(context.Background())

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-interrupt
		log.Info("Received shutdown signal, terminating")
		e.cancel()
		//		e.vdcdClient.Close()
	}()

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

	go e.startDiscovery()
	e.loopVcdcClient()

}

func (e *VcdcBridge) startDiscovery() {
	log.Debugln("Start startDiscovery")

	var (
		tasmotaDiscovery  *discovery.TasmotaDevice
		shellyDiscovery   *discovery.ShellyDevice
		zigbeeDiscovery   *discovery.Zigbee2MQTTDevice
		deconzDiscovery   *discovery.DeconzDevice
		wledDiscovery     *discovery.WledDevice
		homeassistantDisc *discovery.HomeAssistantDevice
	)

	if e.config.mqttDiscoveryEnabled {

		log.WithField("Host", e.config.mqttHost).Info("Connecting to MQTT broker")

		// Connect to MQTT Broker
		if token := e.mqttClient.Connect(); token.Wait() && token.Error() != nil {
			log.WithError(token.Error()).Error("MQTT connect failed")
		}

		defer func() {
			log.Info("Disconnecting MQTT client")
			e.mqttClient.Disconnect(250)
		}()

		// Tasmota Device Discovery
		if !e.config.tasmotaDisabled {
			tasmotaDiscovery = new(discovery.TasmotaDevice)
			go tasmotaDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
		}

		// Shelly Device Discovery
		if !e.config.shellyDisabled {
			shellyDiscovery = new(discovery.ShellyDevice)
			go shellyDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
		}

		// Zigbee2MQTT Discovery
		if !e.config.zigbee2mqttDisabled {
			zigbeeDiscovery = new(discovery.Zigbee2MQTTDevice)
			go zigbeeDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
		}
	}

	if !e.config.deconzDisabled {
		deconzDiscovery = new(discovery.DeconzDevice)
		go deconzDiscovery.StartDiscovery(e.vdcdClient, e.config.deconzHost, e.config.deconzPort, e.config.deconcWebSockerPort, e.config.deconzApi, e.config.deconzEnableGroups)
	}

	// WLED Device Discovery
	if !e.config.wledDisabled {
		wledDiscovery = new(discovery.WledDevice)
		go wledDiscovery.StartDiscovery(e.vdcdClient)
	}

	// Home Assistant Device Discovery
	if !e.config.homeassistantDisabled {
		homeassistantDisc = new(discovery.HomeAssistantDevice)
		go homeassistantDisc.StartDiscovery(e.vdcdClient, e.config.homeassistantURL, e.config.homeassistantToken)
	}

	ticker := time.NewTicker(discoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			log.Info("Discovery loop stopping")
			return
		case <-ticker.C:
			log.WithField("interval", discoveryInterval).Info("Running periodic discovery")

			if !e.config.deconzDisabled && deconzDiscovery != nil {
				go deconzDiscovery.StartDiscovery(e.vdcdClient, e.config.deconzHost, e.config.deconzPort, e.config.deconcWebSockerPort, e.config.deconzApi, e.config.deconzEnableGroups)
			}

			if !e.config.wledDisabled && wledDiscovery != nil {
				go wledDiscovery.StartDiscovery(e.vdcdClient)
			}

			if !e.config.homeassistantDisabled && homeassistantDisc != nil {
				go homeassistantDisc.StartDiscovery(e.vdcdClient, e.config.homeassistantURL, e.config.homeassistantToken)
			}

			if e.config.mqttDiscoveryEnabled {
				if shellyDiscovery != nil {
					go shellyDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
				}
				if zigbeeDiscovery != nil {
					go zigbeeDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
				}
				if tasmotaDiscovery != nil {
					go tasmotaDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
				}
			}
		}
	}
}

func (e *VcdcBridge) loopVcdcClient() {
	e.vdcdClient.ListenWithContext(e.ctx)
}
