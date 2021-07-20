package discovery

import (
	"fmt"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/splattner/vdcd-bridge/pkg/vdcdapi"
)

type GenericDevice struct {
	vdcdClient   *vdcdapi.Client
	mqttClient   mqtt.Client
	originDevice *vdcdapi.Device
}

func (e *GenericDevice) publishMqttCommand(topic string, value interface{}) {
	if token := e.mqttClient.Publish(topic, 0, false, fmt.Sprintf("%v", value)); token.Wait() && token.Error() != nil {
		log.Errorln("MQTT publish failed", token.Error())
	}
}

func (e *GenericDevice) subscribeMqttTopic(topic string, callback mqtt.MessageHandler) {

	log.Debugf("MQTT Subscribe to topic %s\n", topic)
	if token := e.mqttClient.Subscribe(topic, 0, callback); token.Wait() && token.Error() != nil {
		log.Error("MQTT subscribe failed: ", token.Error())
	}
}
