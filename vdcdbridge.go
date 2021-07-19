package main

import (
	"encoding/json"
	"fmt"

	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/splattner/vdcd-bridge/pkg/discovery"
	"github.com/splattner/vdcd-bridge/pkg/vdcdapi"
)

type VcdcBridgeConfig struct {
	host       string
	port       int
	mqttHost   string
	modelName  string
	vendorName string

	mqttDiscoveryEnabled bool
	tasmotaDisabled      bool
	shellyDisabled       bool
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

	// Configure MQTT Client if enabled
	if config.mqttDiscoveryEnabled {
		log.Infof("Connecting to MQTT Host: %s\n", config.mqttHost)

		mqttBroker := fmt.Sprintf("tcp://%s", config.mqttHost)
		opts := mqtt.NewClientOptions().AddBroker(mqttBroker).SetClientID("vdcd_cient")
		opts.SetKeepAlive(60 * time.Second)
		opts.SetDefaultPublishHandler(e.mqttCallback())
		opts.SetPingTimeout(1 * time.Second)
		opts.SetProtocolVersion(3)
		opts.SetOrderMatters(false)

		e.mqttClient = mqtt.NewClient(opts)
	}

	e.wg.Add(2)
	go e.startDiscovery()
	go e.loopVcdcClient()
	e.wg.Wait()

}

func (e *VcdcBridge) startDiscovery() {

	if e.config.mqttDiscoveryEnabled {

		if token := e.mqttClient.Connect(); token.Wait() && token.Error() != nil {
			log.Error("MQTT connect failed: ", token.Error())
		}

		// Tasmota Device Discovery
		if !e.config.tasmotaDisabled {
			if token := e.mqttClient.Subscribe("tasmota/discovery/#", 0, nil); token.Wait() && token.Error() != nil {
				log.Error("MQTT subscribe failed: ", token.Error())
			}
		}

		// Shelly Device Discovery
		if !e.config.shellyDisabled {

			if token := e.mqttClient.Subscribe("shellies/announce", 0, nil); token.Wait() && token.Error() != nil {
				log.Error("MQTT subscribe failed: ", token.Error())
			}

			if token := e.mqttClient.Subscribe("shellies/+/info", 0, nil); token.Wait() && token.Error() != nil {
				log.Error("MQTT subscribe failed: ", token.Error())
			}

			if token := e.mqttClient.Publish("shellies/command", 0, false, "announce"); token.Wait() && token.Error() != nil {
				log.Error("MQTT publish failed: ", token.Error())
			}
		}
	}

	e.wg.Done()

}

func (e *VcdcBridge) loopVcdcClient() {
	e.vcdcClient.Listen()
	e.wg.Done()
}

func (e *VcdcBridge) mqttCallback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		log.Debugf("MQTT Mesage: %s: %s\n", string(msg.Topic()), string(msg.Payload()))

		// Tasmota Device Discovery
		if strings.Contains(msg.Topic(), "tasmota") && strings.Contains(msg.Topic(), "config") {

			tasmotaDevice := new(discovery.TasmotaDevice)
			err := json.Unmarshal(msg.Payload(), &tasmotaDevice)
			if err != nil {
				log.Error("Unmarshal to Tasmota Device failed\n", err.Error())
				return
			}

			tasmotaDevice.NewTasmotaDevice(e.mqttClient)

			log.Infof("Tasmota Device discovered: Name: %s, FriendlyName: %s, IP: %s, Mac %s\n", tasmotaDevice.DeviceName, tasmotaDevice.FriendlyName[0], tasmotaDevice.IPAddress, tasmotaDevice.MACAddress)

			_, notfounderr := e.vcdcClient.GetDeviceByUniqueId(tasmotaDevice.MACAddress)

			if notfounderr != nil {
				// not found
				log.Debugf("Tasmota Device %s not found in vcdc\n", tasmotaDevice.FriendlyName[0])

				log.Debugf("Prepare Device %s\n", tasmotaDevice.FriendlyName[0])
				device := new(vdcdapi.Device)
				device.NewLightDevice(e.vcdcClient, tasmotaDevice.MACAddress, false)
				device.SetName(tasmotaDevice.FriendlyName[0])
				device.SetChannelMessageCB(e.deviceCallback())
				device.ModelName = tasmotaDevice.Module
				device.ModelVersion = tasmotaDevice.SoftwareVersion
				device.SourceDevice = tasmotaDevice

				tasmotaDevice.SetOriginDevice(device)

				// Add callback for this device
				topic := fmt.Sprintf("stat/%s/#", tasmotaDevice.Topic)
				log.Debugf("Subscribe to stats topic %s for device updates\n", topic)
				if token := e.mqttClient.Subscribe(topic, 0, tasmotaDevice.MqttCallback()); token.Wait() && token.Error() != nil {
					log.Error("MQTT subscribe failed: ", token.Error())
				}

				log.Debugf("Adding Tasmota Device %s to vcdc\n", tasmotaDevice.FriendlyName[0])
				e.vcdcClient.AddDevice(device)

			}
		}

		// Shelly Device discovery
		if strings.Contains(msg.Topic(), "shellies") && strings.Contains(msg.Topic(), "announce") {

			shellyDevice := new(discovery.ShellyDevice)
			err := json.Unmarshal(msg.Payload(), &shellyDevice)
			if err != nil {
				log.Errorf("Unmarshal to Shelly Device failed\n", err.Error())
				return
			}

			shellyDevice.NewShellyDevice(e.mqttClient)

			log.Infof("Shelly Device discovered: Name: %s, IP: %s, Mac %s\n", shellyDevice.Id, shellyDevice.IPAddress, shellyDevice.MACAddress)

			_, notfounderr := e.vcdcClient.GetDeviceByUniqueId(shellyDevice.MACAddress)

			if notfounderr != nil {
				// not found
				log.Debugf("Shelly Device not found in vcdc -> Adding \n")

				device := new(vdcdapi.Device)
				device.NewLightDevice(e.vcdcClient, shellyDevice.MACAddress, false)
				device.SetName(shellyDevice.Id)
				device.SetChannelMessageCB(e.deviceCallback())
				device.ModelVersion = shellyDevice.FirmewareVersion
				device.SourceDevice = shellyDevice

				button := new(vdcdapi.Button)
				button.LocalButton = true
				button.Id = "input0"
				button.ButtonType = vdcdapi.SingleButton
				button.Group = vdcdapi.YellowLightGroup
				button.HardwareName = "toggle"

				device.AddButton(*button)

				shellyDevice.SetOriginDevice(device)

				// Add callback for this device
				topic := fmt.Sprintf("shellies/%s/#", shellyDevice.Id)
				if token := e.mqttClient.Subscribe(topic, 0, shellyDevice.MqttCallback()); token.Wait() && token.Error() != nil {
					log.Error("MQTT subscribe failed: ", token.Error())
				}

				e.vcdcClient.AddDevice(device)

			}

		}
		// if strings.Contains(msg.Topic(), "shellies") && strings.Contains(msg.Topic(), "info") {
		// 	log.Println("Shelly info found", string(msg.Payload()))
		// }
	}

	return f
}

func (e *VcdcBridge) deviceCallback() func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {

	var f func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) = func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {
		log.Debugf("Call Back called for Device %s\n", device.UniqueID)

		switch device.SourceDevice.(type) {
		case *discovery.ShellyDevice:

			sourceDevice := device.SourceDevice.(*discovery.ShellyDevice)
			sourceDevice.SetValue(message.Value)
		case *discovery.TasmotaDevice:

			sourceDevice := device.SourceDevice.(*discovery.TasmotaDevice)
			sourceDevice.SetValue(message.Value)
		default:

			log.Errorln("Device not implemented")
		}

	}

	return f
}
