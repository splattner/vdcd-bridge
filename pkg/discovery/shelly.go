package discovery

import (
	"encoding/json"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/splattner/vdcd-bridge/pkg/vdcdapi"
)

type ShellyDevice struct {
	Id                   string `json:"id,omitempty"`
	Model                string `json:"model,omitempty"`
	MACAddress           string `json:"mac,omitempty"`
	IPAddress            string `json:"ip,omitempty"`
	NewFirewareAvailable bool   `json:"new_fw,omitempty"`
	FirmewareVersion     string `json:"fw_ver,omitempty"`

	vdcdClient   *vdcdapi.Client
	mqttClient   mqtt.Client
	originDevice *vdcdapi.Device
}

func (e *ShellyDevice) NewShellyDevice(vdcdClient *vdcdapi.Client, mqttClient mqtt.Client) *vdcdapi.Device {
	e.vdcdClient = vdcdClient
	e.mqttClient = mqttClient
	e.configureCallbacks()

	device := new(vdcdapi.Device)
	device.NewLightDevice(e.vdcdClient, e.MACAddress, false)
	device.SetName(e.Id)
	device.SetChannelMessageCB(e.vcdcChannelCallback())
	device.ModelName = e.Model
	device.ModelVersion = e.FirmewareVersion
	device.SourceDevice = e

	button := new(vdcdapi.Button)
	button.LocalButton = true
	button.Id = "input0"
	button.ButtonType = vdcdapi.SingleButton
	button.Group = vdcdapi.YellowLightGroup
	button.HardwareName = "toggle"

	device.AddButton(*button)

	e.originDevice = device
	e.vdcdClient.AddDevice(device)

	return device

}

// Apply update from dss to shelly
func (e *ShellyDevice) SetValue(value float32, channelName string, channelType vdcdapi.ChannelTypeType) {

	log.Infof("Set Value for Shelly Device %s to %f\n", e.Id, value)

	// Also sync the state with originDevice
	if e.originDevice != nil { // should not happen!
		e.originDevice.SetValue(value, "basic_switch")
	}

	if value == 100 {
		if token := e.mqttClient.Publish("shellies/"+e.Id+"/relay/0/command", 0, false, "on"); token.Wait() && token.Error() != nil {
			log.Errorln("MQTT publish failed", token.Error())
		}
	} else {
		if token := e.mqttClient.Publish("shellies/"+e.Id+"/relay/0/command", 0, false, "off"); token.Wait() && token.Error() != nil {
			log.Errorln("MQTT publish failed", token.Error())
		}
	}

}

func (e *ShellyDevice) StartDiscovery(vdcdClient *vdcdapi.Client, mqttClient mqtt.Client) {
	e.mqttClient = mqttClient
	e.vdcdClient = vdcdClient

	log.Infoln(("Starting Shelly Device discovery"))

	if token := mqttClient.Subscribe("shellies/announce", 0, e.mqttDiscoverCallback()); token.Wait() && token.Error() != nil {
		log.Error("MQTT subscribe failed: ", token.Error())
	}

	if token := mqttClient.Subscribe("shellies/+/info", 0, e.mqttDiscoverCallback()); token.Wait() && token.Error() != nil {
		log.Error("MQTT subscribe failed: ", token.Error())
	}

	if token := mqttClient.Publish("shellies/command", 0, false, "announce"); token.Wait() && token.Error() != nil {
		log.Error("MQTT publish failed: ", token.Error())
	}
}

func (e *ShellyDevice) configureCallbacks() {
	// Add callback for this device
	topic := fmt.Sprintf("shellies/%s/#", e.Id)
	if token := e.mqttClient.Subscribe(topic, 0, e.mqttCallback()); token.Wait() && token.Error() != nil {
		log.Error("MQTT subscribe failed: ", token.Error())
	}
}

func (e *ShellyDevice) mqttCallback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		log.Debugf("Shelly MQTT Message for %s, Topic %s, Message %s", e.Id, string(msg.Topic()), string(msg.Payload()))

		if strings.Contains(msg.Topic(), "relay/0") {
			if strings.Contains(string(msg.Payload()), "on") {
				e.originDevice.UpdateValue(100, "basic_switch", vdcdapi.UndefinedType)
			}
			if strings.Contains(string(msg.Payload()), "off") {
				e.originDevice.UpdateValue(0, "basic_switch", vdcdapi.UndefinedType)
			}
		}

	}

	return f
}

func (e *ShellyDevice) mqttDiscoverCallback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		log.Debugf("MQTT Mesage for Shelly Device discovery: %s: %s\n", string(msg.Topic()), string(msg.Payload()))

		if strings.Contains(msg.Topic(), "announce") {

			shellyDevice := new(ShellyDevice)
			err := json.Unmarshal(msg.Payload(), &shellyDevice)
			if err != nil {
				log.Errorf("Unmarshal to Shelly Device failed\n", err.Error())
				return
			}

			log.Infof("Shelly Device discovered: Name: %s, IP: %s, Mac %s\n", shellyDevice.Id, shellyDevice.IPAddress, shellyDevice.MACAddress)

			_, notfounderr := e.vdcdClient.GetDeviceByUniqueId(shellyDevice.MACAddress)
			if notfounderr != nil {
				log.Debugf("Shelly Device not found in vcdc -> Adding \n")
				shellyDevice.NewShellyDevice(e.vdcdClient, e.mqttClient)
			}

		}
		// if strings.Contains(msg.Topic(), "shellies") && strings.Contains(msg.Topic(), "info") {
		// 	log.Println("Shelly info found", string(msg.Payload()))
		// }
	}

	return f
}

func (e *ShellyDevice) vcdcChannelCallback() func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {

	var f func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) = func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {
		log.Debugf("vcdcCallBack called for Device %s\n", device.UniqueID)
		e.SetValue(message.Value, message.ChannelName, message.ChannelType)
	}

	return f
}
