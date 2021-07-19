package discovery

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/splattner/vdcd-bridge/pkg/vdcdapi"
)

type TasmotaDevice struct {
	IPAddress       string         `json:"ip,omitempty"`
	DeviceName      string         `json:"dn,omitempty"`
	FriendlyName    []string       `json:"fn,omitempty"`
	Hostname        string         `json:"hn,omitempty"`
	MACAddress      string         `json:"mac,omitempty"`
	Module          string         `json:"md,omitempty"`
	TuyaMCUFlag     int            `json:"ty,omitempty"`
	IFAN            int            `json:"if,omitempty"`
	DOffline        string         `json:"ofln,omitempty"`
	DOnline         string         `json:"onln,omitempty"`
	State           []string       `json:"st,omitempty"`
	SoftwareVersion string         `json:"sw,omitempty"`
	Topic           string         `json:"t,omitempty"`
	Fulltopic       string         `json:"ft,omitempty"`
	TopicPrefix     []string       `json:"tp,omitempty"`
	Relays          []int          `json:"rl,omitempty"`
	Switches        []int          `json:"swc,omitempty"`
	SWN             []int          `json:"swn,omitempty"`
	Buttons         []int          `json:"btn,omitempty"`
	SetOptions      map[string]int `json:"so,omitempty"`
	LK              int            `json:"lk,omitempty"`
	LightSubtype    int            `json:"lt_st,omitempty"` // https://github.com/arendst/Tasmota/blob/development/tasmota/xdrv_04_light.ino
	ShutterOptions  []int          `json:"sho,omitempty"`
	Version         int            `json:"ver,omitempty"`

	vdcdClient   *vdcdapi.Client
	mqttClient   mqtt.Client
	originDevice *vdcdapi.Device
}

type TasmotaResultMsg struct {
	Power1   string `json:"POWER1,omitempty"`
	Power    string `json:"POWER,omitempty"`
	Dimmer   int    `json:"Dimmer,omitempty"`
	Color    string `json:"Color,omitempty"`
	HSBCOlor string `json:"HSBColor,omitempty"`
	White    int    `json:"White,omitempty"`
	Channel  []int  `json:"Channel,omitempty"`
}

func (e *TasmotaDevice) NewTasmotaDevice(vdcdClient *vdcdapi.Client, mqttClient mqtt.Client) *vdcdapi.Device {
	e.vdcdClient = vdcdClient
	e.mqttClient = mqttClient
	e.configureCallbacks()

	device := new(vdcdapi.Device)

	switch e.LightSubtype {
	case 0:
		// Sonoff Basic
		device.NewLightDevice(e.vdcdClient, e.MACAddress, false)
	case 4:
		// RGBW
		device.NewColorLightDevice(e.vdcdClient, e.MACAddress)
	default:
		device.NewLightDevice(e.vdcdClient, e.MACAddress, false)
	}

	device.SetName(e.FriendlyName[0])
	device.SetChannelMessageCB(e.vcdcChannelCallback())
	device.ModelName = e.Module
	device.ModelVersion = e.SoftwareVersion
	device.SourceDevice = e

	e.originDevice = device

	log.Debugf("Adding Tasmota Device %s to vcdc\n", e.FriendlyName[0])
	e.vdcdClient.AddDevice(device)

	return device

}

// Apply update from dss to shelly
func (e *TasmotaDevice) SetValue(value float32, channelName string, channelType vdcdapi.ChannelTypeType) {

	log.Infof("Set Value Tasmota Device %s %s to %f on Channel '%s' \n", e.DeviceName, e.FriendlyName[0], value, channelName)

	// Also sync the state with originDevice
	if e.originDevice != nil { // should not happen!
		e.originDevice.SetValue(value, channelName)
	}

	switch channelName {
	case "basic_switch":
		if value == 100 {
			if token := e.mqttClient.Publish("cmnd/"+e.Topic+"/POWER", 0, false, "on"); token.Wait() && token.Error() != nil {
				log.Errorln("MQTT publish failed", token.Error())
			}
		} else {
			if token := e.mqttClient.Publish("cmnd/"+e.Topic+"/POWER", 0, false, "off"); token.Wait() && token.Error() != nil {
				log.Errorln("MQTT publish failed", token.Error())
			}
		}

	case "brightness":
		if token := e.mqttClient.Publish("cmnd/"+e.Topic+"/HsbColor3", 0, false, fmt.Sprintf("%f", value)); token.Wait() && token.Error() != nil {
			log.Errorln("MQTT publish failed", token.Error())
		}

	case "hue":
		if token := e.mqttClient.Publish("cmnd/"+e.Topic+"/HsbColor1", 0, false, fmt.Sprintf("%f", value)); token.Wait() && token.Error() != nil {
			log.Errorln("MQTT publish failed", token.Error())
		}

	case "saturation":
		if token := e.mqttClient.Publish("cmnd/"+e.Topic+"/HsbColor2", 0, false, fmt.Sprintf("%f", value)); token.Wait() && token.Error() != nil {
			log.Errorln("MQTT publish failed", token.Error())
		}

	}

}

func (e *TasmotaDevice) StartDiscovery(vdcdClient *vdcdapi.Client, mqttClient mqtt.Client) {
	e.mqttClient = mqttClient
	e.vdcdClient = vdcdClient

	log.Infoln(("Starting Tasmota Device discovery"))

	if token := mqttClient.Subscribe("tasmota/discovery/#", 0, e.mqttDiscoverCallback()); token.Wait() && token.Error() != nil {
		log.Error("MQTT subscribe failed: ", token.Error())
	}

}

func (e *TasmotaDevice) configureCallbacks() {
	// Add callback for this device
	topic := fmt.Sprintf("stat/%s/#", e.Topic)
	log.Debugf("Subscribe to stats topic %s for device updates\n", topic)
	if token := e.mqttClient.Subscribe(topic, 0, e.mqttCallback()); token.Wait() && token.Error() != nil {
		log.Error("MQTT subscribe failed: ", token.Error())
	}
}

// MQTT Callback from tasmota device
// This updates the dss channel on the linked origin vdcd-brige Dev ice
func (e *TasmotaDevice) mqttCallback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		log.Debugf("Tasmota MQTT Message for %s, Topic %s, Message %s", e.DeviceName, string(msg.Topic()), string(msg.Payload()))

		if strings.Contains(msg.Topic(), "RESULT") {

			var resultMesage TasmotaResultMsg
			err := json.Unmarshal(msg.Payload(), &resultMesage)
			if err != nil {
				log.Errorf("Unmarshal to TasmotaPowerMsg failed\n", err.Error())
				return
			}

			if resultMesage.Power1 == "ON" || resultMesage.Power == "ON" {
				e.originDevice.UpdateValue(100, "basic_switch", vdcdapi.UndefinedType)
			}
			if resultMesage.Power1 == "OFF" || resultMesage.Power == "ON" {
				e.originDevice.UpdateValue(0, "basic_switch", vdcdapi.UndefinedType)
			}

			if resultMesage.HSBCOlor != "" {
				hsbcolor := strings.Split(resultMesage.HSBCOlor, ",")

				if hue, err := strconv.ParseFloat(hsbcolor[0], 32); err == nil {
					e.originDevice.UpdateValue(float32(hue), "hue", vdcdapi.HueType)
				}

				if saturation, err := strconv.ParseFloat(hsbcolor[1], 32); err == nil {
					e.originDevice.UpdateValue(float32(saturation), "saturation", vdcdapi.SaturationType)
				}

				if brightness, err := strconv.ParseFloat(hsbcolor[2], 32); err == nil {
					e.originDevice.UpdateValue(float32(brightness), "brightness", vdcdapi.BrightnessType)
				}

			}
		}

	}

	return f
}

func (e *TasmotaDevice) mqttDiscoverCallback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		log.Debugf("MQTT Mesage for Tasmota Device Discovery: %s: %s\n", string(msg.Topic()), string(msg.Payload()))

		if strings.Contains(msg.Topic(), "config") {

			tasmotaDevice := new(TasmotaDevice)
			err := json.Unmarshal(msg.Payload(), &tasmotaDevice)
			if err != nil {
				log.Error("Unmarshal to Tasmota Device failed\n", err.Error())
				return
			}

			log.Infof("Tasmota Device discovered: Name: %s, FriendlyName: %s, IP: %s, Mac %s\n", tasmotaDevice.DeviceName, tasmotaDevice.FriendlyName[0], tasmotaDevice.IPAddress, tasmotaDevice.MACAddress)

			_, notfounderr := e.vdcdClient.GetDeviceByUniqueId(tasmotaDevice.MACAddress)
			if notfounderr != nil {
				log.Debugf("Tasmota Device %s not found in vcdc\n", tasmotaDevice.FriendlyName[0])
				tasmotaDevice.NewTasmotaDevice(e.vdcdClient, e.mqttClient)
			}
		}

	}

	return f
}

func (e *TasmotaDevice) vcdcChannelCallback() func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {

	var f func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) = func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {
		log.Debugf("vcdcCallBack called for Device %s\n", device.UniqueID)
		e.SetValue(message.Value, message.ChannelName, message.ChannelType)
	}

	return f
}
