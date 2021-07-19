package discovery

import (
	"encoding/json"
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
	LightSubtype    int            `json:"lt_st,omitempty"`
	ShutterOptions  []int          `json:"sho,omitempty"`
	Version         int            `json:"ver,omitempty"`

	mqttClient   mqtt.Client
	originDevice *vdcdapi.Device
}

type TasmotaPowerMsg struct {
	Power1 string `json:"POWER1,omitempty"`
}

func (e *TasmotaDevice) NewTasmotaDevice(mqttClient mqtt.Client) {
	e.mqttClient = mqttClient

}

func (e *TasmotaDevice) SetOriginDevice(originDevice *vdcdapi.Device) {
	e.originDevice = originDevice
}

// Apply update from dss to shelly
func (e *TasmotaDevice) SetValue(value float32) {

	log.Infof("Set Value for Tasmota Device %s %s to %f\n", e.DeviceName, e.FriendlyName[0], value)

	// Also sync the state with originDevice
	if e.originDevice != nil { // should not happen!
		e.originDevice.SetValue(value)
	}

	if value == 100 {
		if token := e.mqttClient.Publish("cmnd/"+e.Topic+"/POWER", 0, false, "on"); token.Wait() && token.Error() != nil {
			log.Errorln("MQTT publish failed", token.Error())
		}
	} else {
		if token := e.mqttClient.Publish("cmnd/"+e.Topic+"/POWER", 0, false, "off"); token.Wait() && token.Error() != nil {
			log.Errorln("MQTT publish failed", token.Error())
		}
	}

}

func (e *TasmotaDevice) MqttCallback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		log.Debugf("Tasmota MQTT Message for %s, Topic %s, Message %s", e.DeviceName, string(msg.Topic()), string(msg.Payload()))

		if strings.Contains(msg.Topic(), "RESULT") {

			var powerMesage TasmotaPowerMsg
			err := json.Unmarshal(msg.Payload(), &powerMesage)
			if err != nil {
				log.Errorf("Unmarshal to TasmotaPowerMsg failed\n", err.Error())
				return
			}

			if powerMesage.Power1 == "ON" {
				e.originDevice.UpdateValue(100, "basic switch", vdcdapi.UndefinedType)
			}
			if powerMesage.Power1 == "OFF" {
				e.originDevice.UpdateValue(0, "basic switch", vdcdapi.UndefinedType)
			}
		}

	}

	return f
}
