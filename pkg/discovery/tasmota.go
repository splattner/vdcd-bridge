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
	GenericDevice
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
	LK              int            `json:"lk,omitempty"`    // LightColor (LC) and RGB LinKed https://github.com/arendst/Tasmota/blob/development/tasmota/xdrv_04_light.ino#L689
	LightSubtype    int            `json:"lt_st,omitempty"` // https://github.com/arendst/Tasmota/blob/development/tasmota/xdrv_04_light.ino
	ShutterOptions  []int          `json:"sho,omitempty"`
	Version         int            `json:"ver,omitempty"`
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

type TasmotaTeleMsg struct {
	Time     string              `json:"time,omitempty"`
	TempUnit string              `json:"TempUnit,omitempty"`
	SI7021   TasmotaTeleSI721Msg `json:"SI7021,omitempty"`
}

type TasmotaTeleSI721Msg struct {
	Temperature float32 `json:"Temperature,omitempty"`
	Humidity    float32 `json:"Humidity,omitempty"`
	DewPoint    float32 `json:"DewPoint,omitempty"`
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

	device.ConfigUrl = fmt.Sprintf("http://%s", e.IPAddress)

	e.originDevice = device

	// Sensor
	temperaturSensor := new(vdcdapi.Sensor)
	temperaturSensor.SensorType = vdcdapi.TemperatureSensor
	temperaturSensor.Usage = vdcdapi.RoomSensorUsageType
	temperaturSensor.Id = fmt.Sprintf("%s-temperature", device.UniqueID)
	temperaturSensor.Resolution = 0.1
	temperaturSensor.UpdateInterval = 0 // no fixed interval

	humiditySensor := new(vdcdapi.Sensor)
	humiditySensor.SensorType = vdcdapi.HumiditySensor
	humiditySensor.Usage = vdcdapi.RoomSensorUsageType
	humiditySensor.Id = fmt.Sprintf("%s-humidity", device.UniqueID)
	humiditySensor.Resolution = 0.1
	humiditySensor.UpdateInterval = 0 // no fixed interval

	device.AddSensor(*temperaturSensor)
	device.AddSensor(*humiditySensor)

	log.Debugf("Adding Tasmota Device %s to vcdc\n", e.FriendlyName[0])
	e.vdcdClient.AddDevice(device)

	return device

}

// Apply update from dss to shelly
func (e *TasmotaDevice) SetValue(value float32, channelName string, channelType vdcdapi.ChannelTypeType) {

	log.Infof("Set Value Tasmota Device %s %s to %f on Channel '%s' \n", e.DeviceName, e.FriendlyName[0], value, channelName)

	// Also sync the state with originDevice
	e.originDevice.SetValue(value, channelName)

	switch channelName {
	case "basic_switch":
		if value == 100 {
			e.TurnOn()
		} else {
			e.TurnOff()
		}

	case "brightness", "hue", "saturation":

		// Get all values as they are dependent
		brightness, _ := e.originDevice.GetValue("brightness")
		hue, _ := e.originDevice.GetValue("hue")
		saturation, _ := e.originDevice.GetValue("saturation")

		if e.LightSubtype == 4 && saturation == 0 {
			if saturation == 0 {
				e.SetWhite(brightness)
			} else {
				e.SetHSB(hue, saturation, brightness)
			}

		} else {
			e.SetBrightness(brightness)
		}

	case "colortemp":
		e.SetColorTemp(value)

	}

}

func (e *TasmotaDevice) StartDiscovery(vdcdClient *vdcdapi.Client, mqttClient mqtt.Client) {
	e.mqttClient = mqttClient
	e.vdcdClient = vdcdClient

	log.Infoln(("Starting Tasmota Device discovery"))
	e.subscribeMqttTopic("tasmota/discovery/#", e.mqttDiscoverCallback())
}

func (e *TasmotaDevice) configureCallbacks() {
	// Add callback for stat
	topicStat := fmt.Sprintf("stat/%s/#", e.Topic)
	e.subscribeMqttTopic(topicStat, e.mqttCallback())

	// Add callback for tele
	topicTele := fmt.Sprintf("tele/%s/#", e.Topic)
	e.subscribeMqttTopic(topicTele, e.mqttCallback())
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
				log.WithError(err).Error("Unmarshal to TasmotaPowerMsg failed")
				return
			}

			if resultMesage.Power1 == "ON" || resultMesage.Power == "ON" {
				e.originDevice.UpdateValue(100, "basic_switch", vdcdapi.UndefinedType)
			}
			if resultMesage.Power1 == "OFF" || resultMesage.Power == "ON" {
				e.originDevice.UpdateValue(0, "basic_switch", vdcdapi.UndefinedType)
			}

			if resultMesage.HSBCOlor != "" && resultMesage.White == 0 {
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

			if resultMesage.White > 0 {
				//e.originDevice.UpdateValue(float32(0), "hue", vdcdapi.HueType)
				e.originDevice.UpdateValue(float32(0), "saturation", vdcdapi.SaturationType)
				e.originDevice.UpdateValue(float32(resultMesage.White), "brightness", vdcdapi.BrightnessType)

			}
		}

		if strings.Contains(msg.Topic(), "SENSOR") {
			var teleMsg TasmotaTeleMsg
			err := json.Unmarshal(msg.Payload(), &teleMsg)
			if err != nil {
				log.WithError(err).Error("Unmarshal to TasmotaTeleMsg failed")
				return
			}

			e.originDevice.UpdateSensorValue(teleMsg.SI7021.Temperature, fmt.Sprintf("%s-temperature", e.originDevice.UniqueID))
			e.originDevice.UpdateSensorValue(teleMsg.SI7021.Humidity, fmt.Sprintf("%s-humidity", e.originDevice.UniqueID))

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

func (e *TasmotaDevice) TurnOn() {
	e.publishMqttCommand("cmnd/"+e.Topic+"/POWER", "on")
}

func (e *TasmotaDevice) TurnOff() {
	e.publishMqttCommand("cmnd/"+e.Topic+"/POWER", "off")
}

func (e *TasmotaDevice) SetBrightness(brightness float32) {
	e.publishMqttCommand("cmnd/"+e.Topic+"/HsbColor3", brightness)
}

func (e *TasmotaDevice) SetHue(hue float32) {
	e.publishMqttCommand("cmnd/"+e.Topic+"/HsbColor1", hue)
}

func (e *TasmotaDevice) SetSaturation(saturation float32) {
	e.publishMqttCommand("cmnd/"+e.Topic+"/HsbColor2", saturation)
}

func (e *TasmotaDevice) SetHSB(hue float32, saturation float32, brightness float32) {
	e.publishMqttCommand("cmnd/"+e.Topic+"/HsbColor", fmt.Sprintf("%.0f,%.0f,%.0f", hue, saturation, brightness))
}

func (e *TasmotaDevice) SetWhite(white float32) {
	//e.publishMqttCommand("cmnd/"+e.Topic+"/Color1", "0,0,0")
	e.publishMqttCommand("cmnd/"+e.Topic+"/White", white)
}

func (e *TasmotaDevice) SetColorTemp(ct float32) {
	log.Warningln("Setting Color Temp not implemented")
}
