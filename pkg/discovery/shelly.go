package discovery

import (
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type ShellyDevice struct {
	Id                   string `json:"id,omitempty"`
	Model                string `json:"model,omitempty"`
	MACAddress           string `json:"mac,omitempty"`
	IPAddress            string `json:"ip,omitempty"`
	NewFirewareAvailable bool   `json:"new_fw,omitempty"`
	FirmewareVersion     string `json:"fw_ver,omitempty"`

	mqttClient mqtt.Client
}

func (e *ShellyDevice) NewShellyDevice(mqttClient mqtt.Client) {
	e.mqttClient = mqttClient

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
