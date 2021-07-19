package discovery

import (
	"log"
	"strings"

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

	mqttClient   mqtt.Client
	originDevice vdcdapi.Device
}

func (e *ShellyDevice) NewShellyDevice(mqttClient mqtt.Client) {
	e.mqttClient = mqttClient

}

func (e *ShellyDevice) SetOriginDevice(originDevice vdcdapi.Device) {
	e.originDevice = originDevice
}

func (e *ShellyDevice) SetValue(value float32) {

	if value == 100 {
		if token := e.mqttClient.Publish("shellies/"+e.Id+"/relay/0/command", 0, false, "on"); token.Wait() && token.Error() != nil {
			log.Println(token.Error())
		}
	} else {
		if token := e.mqttClient.Publish("shellies/"+e.Id+"/relay/0/command", 0, false, "off"); token.Wait() && token.Error() != nil {
			log.Println(token.Error())
		}
	}

}

func (e *ShellyDevice) MqttCallback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		log.Printf("Shelly MQTT Message for %s, Topic %s, Message %s", e.Id, string(msg.Topic()), string(msg.Payload()))

		if strings.Contains(msg.Topic(), "relay/0") {
			if strings.Contains(string(msg.Payload()), "on") {
				e.originDevice.UpdateValue(100)
			}
			if strings.Contains(string(msg.Payload()), "off") {
				e.originDevice.UpdateValue(0)
			}
		}

	}

	return f
}
