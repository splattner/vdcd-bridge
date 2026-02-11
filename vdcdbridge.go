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
	defer e.vdcdClient.Close()

	e.ctx, e.cancel = context.WithCancel(context.Background())

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-interrupt
		log.Info("Received shutdown signal, terminating")
		e.cancel()
		e.vdcdClient.Close()
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

	e.wg.Add(2)
	go e.startDiscovery()
	go e.loopVcdcClient()

	log.Debug("Waiting for Waitgroup")
	e.wg.Wait()
	log.Debug("Waitgroup finished")
}

func (e *VcdcBridge) startDiscovery() {
	log.Debugln("Start startDiscovery")
	defer e.wg.Done()

	var (
		tasmotaDiscovery  *discovery.TasmotaDevice
		shellyDiscovery   *discovery.ShellyDevice
		zigbeeDiscovery   *discovery.Zigbee2MQTTDevice
		deconzDiscovery   *discovery.DeconzDevice
		wledDiscovery     *discovery.WledDevice
		homeassistantDisc *discovery.HomeAssistantDevice
	)

	if e.config.mqttDiscoveryEnabled {

		log.Info("MQTT connect")

		// Connect to MQTT Broker
		if token := e.mqttClient.Connect(); token.Wait() && token.Error() != nil {
			log.WithError(token.Error()).Error("MQTT connect failed")
		}

		// Tasmota Device Discovery
		if !e.config.tasmotaDisabled {
			tasmotaDiscovery = new(discovery.TasmotaDevice)
			tasmotaDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
		}

		// Shelly Device Discovery
		if !e.config.shellyDisabled {
			shellyDiscovery = new(discovery.ShellyDevice)
			shellyDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
		}

		// Zigbee2MQTT Discovery
		if !e.config.zigbee2mqttDisabled {
			zigbeeDiscovery = new(discovery.Zigbee2MQTTDevice)
			zigbeeDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
		}
	}

	if !e.config.deconzDisabled {
		deconzDiscovery = new(discovery.DeconzDevice)
		deconzDiscovery.StartDiscovery(e.vdcdClient, e.config.deconzHost, e.config.deconzPort, e.config.deconcWebSockerPort, e.config.deconzApi, e.config.deconzEnableGroups)
	}

	// WLED Device Discovery
	if !e.config.wledDisabled {
		wledDiscovery = new(discovery.WledDevice)
		wledDiscovery.StartDiscovery(e.vdcdClient)
	}

	// Home Assistant Device Discovery
	if !e.config.homeassistantDisabled {
		homeassistantDisc = new(discovery.HomeAssistantDevice)
		homeassistantDisc.StartDiscovery(e.vdcdClient, e.config.homeassistantURL, e.config.homeassistantToken)
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
				deconzDiscovery.StartDiscovery(e.vdcdClient, e.config.deconzHost, e.config.deconzPort, e.config.deconcWebSockerPort, e.config.deconzApi, e.config.deconzEnableGroups)
			}

			if !e.config.wledDisabled && wledDiscovery != nil {
				wledDiscovery.StartDiscovery(e.vdcdClient)
			}

			if !e.config.homeassistantDisabled && homeassistantDisc != nil {
				homeassistantDisc.StartDiscovery(e.vdcdClient, e.config.homeassistantURL, e.config.homeassistantToken)
			}

			if e.config.mqttDiscoveryEnabled {
				if shellyDiscovery != nil {
					shellyDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
				}
				if zigbeeDiscovery != nil {
					zigbeeDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
				}
				if tasmotaDiscovery != nil {
					tasmotaDiscovery.StartDiscovery(e.vdcdClient, e.mqttClient)
				}
			}
		}
	}

	log.Debug("Calling Waitgroup done for startDiscovery")
}

func (e *VcdcBridge) loopVcdcClient() {
	defer e.wg.Done()
	log.Debug("Start loopVcdcClient")
	e.vdcdClient.ListenWithContext(e.ctx)

	log.Debug("Calling Waitgroup done for loopVcdcClient")
}
