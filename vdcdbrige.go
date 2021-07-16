package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/splattner/go-vdcd-api-client/pkg/discovery"
	"github.com/splattner/go-vdcd-api-client/pkg/vdcdapi"
)

type VcdcBridgeConfig struct {
	host       string
	port       int
	mqttHost   string
	modelName  string
	vendorName string

	mqttDiscoveryEnabled bool
}

type VcdcBridge struct {
	vcdcClient *vdcdapi.Client
	mqttClient mqtt.Client
	wg         sync.WaitGroup

	config VcdcBridgeConfig
}

func (e *VcdcBridge) NewVcdcBrige(config VcdcBridgeConfig) {

	e.config = config

	e.vcdcClient = new(vdcdapi.Client)

	e.vcdcClient.NewCient(e.config.host, e.config.port, e.config.modelName, e.config.vendorName)
	e.vcdcClient.Connect()
	defer e.vcdcClient.Close()

	if config.mqttHost != "" {
		log.Printf("Connecting to MQTT Host: %s\n", config.mqttHost)

		mqttBroker := fmt.Sprintf("tcp://%s", config.mqttHost)
		opts := mqtt.NewClientOptions().AddBroker(mqttBroker).SetClientID("vdcd_cient")
		opts.SetKeepAlive(60 * time.Second)
		opts.SetDefaultPublishHandler(e.mqttCallback())
		opts.SetPingTimeout(1 * time.Second)

		e.mqttClient = mqtt.NewClient(opts)
	}

	e.wg.Add(1)
	go e.startDiscovery()
	e.wg.Add(1)
	go e.loopVcdcClient()
	e.wg.Wait()

}

func (e *VcdcBridge) startDiscovery() {

	if e.config.mqttDiscoveryEnabled {

		if token := e.mqttClient.Connect(); token.Wait() && token.Error() != nil {
			log.Printf("MQTT failed\n")
			log.Println(token.Error())
		}

		// Tasmota Device Discovery
		if token := e.mqttClient.Subscribe("tasmota/discovery/#", 0, nil); token.Wait() && token.Error() != nil {
			log.Println(token.Error())
		}
	}

}

func (e *VcdcBridge) loopVcdcClient() {
	go e.vcdcClient.Listen()

}

func (e *VcdcBridge) mqttCallback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		// Tasmota Device Discovery
		if strings.Contains(msg.Topic(), "tasmota") && strings.Contains(msg.Topic(), "config") {

			var tasmotaDevice discovery.TasmotaDevice
			err := json.Unmarshal(msg.Payload(), &tasmotaDevice)
			if err != nil {
				log.Print("Unmarshal to Tasmota Device failed\n", err.Error())
				return
			}

			log.Printf("Tasmota Device: Name: %s, FriendlyName: %s, IP: %s, Mac %s\n", tasmotaDevice.DeviceName, tasmotaDevice.FriendlyName[0], tasmotaDevice.IPAddress, tasmotaDevice.MACAddress)

			_, notfounderr := e.vcdcClient.GetDeviceByUniqueId(tasmotaDevice.MACAddress)

			if notfounderr != nil {
				// not found
				log.Printf("Device not found in vcdc -> Adding \n")

				device := vdcdapi.Device{}
				device.NewDevice(*e.vcdcClient, tasmotaDevice.MACAddress)
				device.SetName(tasmotaDevice.FriendlyName[0])
				device.SetChannelMessageCB(e.deviceCallback())
				device.ModelName = tasmotaDevice.Module
				device.ModelVersion = tasmotaDevice.SoftwareVersion
				device.SetChannelMessageCB(e.deviceCallback())
				device.SourceDevice = tasmotaDevice

				e.vcdcClient.AddDevice(device)
			}
		}
	}

	return f
}

func (e *VcdcBridge) deviceCallback() func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {

	var f func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) = func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {
		log.Printf("Call Back called for Device %s\n", device.UniqueID)

		sourceDevice := device.SourceDevice.(discovery.TasmotaDevice)
		sourceDevice.SetValue(message.Value)
	}

	return f
}
