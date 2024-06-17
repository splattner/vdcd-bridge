package discovery

import (
	"encoding/json"
	"fmt"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	log "github.com/sirupsen/logrus"
	"github.com/splattner/vdcd-bridge/pkg/vdcdapi"
)

type Zigbee2MQTTDevice struct {
	GenericDevice

	z2MGroup  Z2MGroup
	z2MDevice Z2MDevice

	IsDevice bool
	IsGroup  bool

	Topic        string
	FriendlyName string

	// For when there are multiple buttons on one device
	// action is the identifier to get the correct device
	actionID     int
	actionPrefix string

	mqttProxy *MQTTProxy
}

// This MQTT Proxy is needed because we can only subscribe to a topic once with a callback.
// The second subscribe would overwrite the registered callback function
type MQTTProxy struct {
	mqttClient mqtt.Client
	receivers  map[string][]mqtt.MessageHandler
}

type Z2MEndpoint struct {
	Bindings             []interface{} `json:"bindings"`
	ConfiguredReportings []interface{} `json:"configured_reportings"`
	Clusters             Z2MCluster    `json:"clusters"`
}

type Z2MCluster struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type Z2MScene struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Z2MDefinition struct {
	Model       string        `json:"model"`
	Vendor      string        `json:"vendor"`
	Description string        `json:"description"`
	Options     []interface{} `json:"options"` // Customize the type according to actual options structure
	Exposes     []Z2MFeatures `json:"exposes"` // Customize the type according to actual exposes structure
}

type Z2MFeatures struct {
	Features    []Z2MFeatures `json:"features,omitempty"`
	Access      int           `json:"access,omitempty"`
	Description string        `json:"description,omitempty"`
	Label       string        `json:"label,omitempty"`
	Name        string        `json:"name,omitempty"`
	Property    string        `json:"property,omitempty"`
	Type        string        `json:"type,omitempty"` // Light, Enum, numeric, binary
	Unit        string        `json:"unit,omitempty"`
	ValueOff    string        `json:"value_off,omitempty"`
	ValueOn     string        `json:"value_on,omitempty"`
	ValueToggle string        `json:"value_toggle,omitempty"`
	ValueMax    int           `json:"value_max,omitempty"`
	ValueMin    int           `json:"value_min,omitempty"`
	Values      []string      `json:"values,omitempty"`
}

type Z2MDevice struct {
	IEEEAddress        string                 `json:"ieee_address"`
	Type               string                 `json:"type"`
	NetworkAddress     int                    `json:"network_address"`
	Supported          bool                   `json:"supported"`
	Disabled           bool                   `json:"disabled"`
	FriendlyName       string                 `json:"friendly_name"`
	Description        string                 `json:"description"`
	Endpoints          map[string]Z2MEndpoint `json:"endpoints"`
	Definition         Z2MDefinition          `json:"definition"`
	PowerSource        string                 `json:"power_source"`
	DateCode           string                 `json:"date_code"`
	ModelID            string                 `json:"model_id"`
	Scenes             []Z2MScene             `json:"scenes"`
	Interviewing       bool                   `json:"interviewing"`
	InterviewCompleted bool                   `json:"interview_completed"`
}

type Z2MMember struct {
	IEEEAddress string `json:"ieee_address"`
	Endpoint    int    `json:"endpoint"`
}

type Z2MGroup struct {
	ID           int         `json:"id"`
	FriendlyName string      `json:"friendly_name"`
	Scenes       []Z2MScene  `json:"scenes"`
	Members      []Z2MMember `json:"members"`
}

type Z2MDeviceData struct {
	Battery           *int     `json:"battery,omitempty"`
	BatteryLow        *bool    `json:"battery_low,omitempty"`
	Humidity          *float32 `json:"humidity,omitempty"`
	LinkQuality       *int     `json:"linkquality,omitempty"`
	PowerOutageCount  *int     `json:"power_outage_count,omitempty"`
	Pressure          *float32 `json:"pressure,omitempty"`
	Temperature       *float32 `json:"temperature,omitempty"`
	DeviceTemperature *float32 `json:"device_temperature,omitempty"`
	Voltage           *float32 `json:"voltage,omitempty"`
	Contact           *bool    `json:"contact,omitempty"`
	TriggerCount      *int     `json:"trigger_count,omitempty"`
	Action            *string  `json:"action,omitempty"`
	OperationMode     *string  `json:"operation_mode,omitempty"`
	Sensitivity       *string  `json:"sensitivity,omitempty"`
	Tamper            *bool    `json:"tamper,omitempty"`
	KeepTime          *int     `json:"keep_time,omitempty"`
	Brightness        *float32 `json:"brightness,omitempty"`
	PowerOnBehaviour  *string  `json:"power_on_behaviour,omitempty"`
	State             *string  `json:"state,omitempty"`
	UpdateAvailable   *bool    `json:"update_available,omitempty"`
	ColorMode         *string  `json:"color_mode,omitempty"`
	ColorTemp         *int     `json:"color_temp,omitempty"`
}

func (e *Zigbee2MQTTDevice) NewZigbee2MQTT(vdcdClient *vdcdapi.Client, mqttClient mqtt.Client, device *vdcdapi.Device) {
	e.vdcdClient = vdcdClient
	e.mqttClient = mqttClient

	if device == nil {
		device = new(vdcdapi.Device)
		device.SetChannelMessageCB(e.vcdcChannelCallback())
		device.SourceDevice = e
		e.originDevice = device
	}

	if e.IsDevice {
		// The topic is set to the IEEEAddress if no Friendly Name is given
		e.Topic = e.z2MDevice.IEEEAddress
		if e.z2MDevice.FriendlyName != "" {
			e.Topic = e.z2MDevice.FriendlyName
		}

		device.ModelName = fmt.Sprintf("%s %s %s", e.z2MDevice.Definition.Vendor, e.z2MDevice.ModelID, e.z2MDevice.Definition.Model)
	}

	if e.IsGroup {
		e.Topic = e.z2MGroup.FriendlyName
		device.NewLightDevice(e.vdcdClient, e.z2MGroup.FriendlyName, false)
		// TODO: only add lights for the moment
		return

	}

	e.FriendlyName = e.getFriendlyName()
	e.configureCallbacks()
	device.SetName(e.FriendlyName)

	log.WithFields(log.Fields{
		"FriendlyName": e.FriendlyName,
		"UniqueID":     e.getUniqueId(),
	}).Debug("Adding Zigbee2MQTT Device or Group to vcdc")
	e.vdcdClient.AddDevice(device)

}

func (e *Zigbee2MQTTDevice) CreateButtonDevice(z2mDevice Z2MDevice, actionID int, actionPrefix string) *Zigbee2MQTTDevice {

	log.WithFields(log.Fields{
		"IEEEAddress":  z2mDevice.IEEEAddress,
		"ActionPrefix": actionPrefix,
	}).Info("Create Z2M ButtonDevice")

	zigbee2mqttdevice := new(Zigbee2MQTTDevice)
	zigbee2mqttdevice.z2MDevice = z2mDevice
	zigbee2mqttdevice.IsDevice = true
	zigbee2mqttdevice.actionID = actionID
	zigbee2mqttdevice.actionPrefix = actionPrefix
	zigbee2mqttdevice.mqttProxy = e.mqttProxy

	device := new(vdcdapi.Device)
	device.SetChannelMessageCB(zigbee2mqttdevice.vcdcChannelCallback())
	device.SourceDevice = zigbee2mqttdevice
	zigbee2mqttdevice.originDevice = device

	device.NewButtonDevice(e.vdcdClient, zigbee2mqttdevice.getUniqueId())

	button := new(vdcdapi.Button)
	button.Id = fmt.Sprintf("button%d", e.actionID+1)
	button.ButtonId = e.actionID
	button.ButtonType = vdcdapi.SingleButton
	button.Group = vdcdapi.YellowLightGroup
	button.LocalButton = false
	button.HardwareName = e.actionPrefix

	device.AddButton(*button)

	_, notfounderr := e.vdcdClient.GetDeviceByUniqueId(zigbee2mqttdevice.getUniqueId())
	if notfounderr != nil {
		log.WithFields(log.Fields{
			"IEEEAddress": z2mDevice.IEEEAddress,
			"UniqueID":    zigbee2mqttdevice.getUniqueId(),
		}).Debug("Z2MDevice not found in vcdc -> Adding")
		zigbee2mqttdevice.NewZigbee2MQTT(e.vdcdClient, e.mqttClient, device)
	}

	return zigbee2mqttdevice

}

func (e *Zigbee2MQTTDevice) CreateLightDevice(z2mDevice Z2MDevice) {

	log.WithFields(log.Fields{
		"IEEEAddress": z2mDevice.IEEEAddress,
	}).Info("Create Z2M Light Device")

	zigbee2mqttdevice := new(Zigbee2MQTTDevice)
	zigbee2mqttdevice.z2MDevice = z2mDevice
	zigbee2mqttdevice.IsDevice = true
	zigbee2mqttdevice.mqttProxy = e.mqttProxy

	device := new(vdcdapi.Device)
	device.SetChannelMessageCB(zigbee2mqttdevice.vcdcChannelCallback())
	device.SourceDevice = zigbee2mqttdevice
	zigbee2mqttdevice.originDevice = device

	var hasState, hasBrighness, hasColorTemp bool

	for _, feature := range e.z2MDevice.Definition.Exposes {
		switch feature.Type {
		case "light":
			for _, lightFeature := range feature.Features {
				switch lightFeature.Property {
				case "state":
					hasState = true
				case "brightness":
					hasBrighness = true
				case "color_temp":
					hasColorTemp = true
				}
			}
		}
	}

	if hasState && hasBrighness && !hasColorTemp {
		device.NewLightDevice(e.vdcdClient, e.z2MDevice.IEEEAddress, true)
	}

	if hasState && hasBrighness && hasColorTemp {
		device.NewCTLightDevice(e.vdcdClient, e.z2MDevice.IEEEAddress)
	}

	_, notfounderr := e.vdcdClient.GetDeviceByUniqueId(e.getUniqueId())
	if notfounderr != nil {
		zigbee2mqttdevice.NewZigbee2MQTT(e.vdcdClient, e.mqttClient, device)
	}

}

func (e *Zigbee2MQTTDevice) getUniqueId() string {
	var uniqueID string

	if e.IsDevice {
		if e.actionPrefix != "" {
			uniqueID = fmt.Sprintf("%s-%d", e.z2MDevice.IEEEAddress, e.actionID)
		} else {
			uniqueID = e.z2MDevice.IEEEAddress
		}
	}

	if e.IsGroup {
		uniqueID = e.z2MGroup.FriendlyName
	}

	return uniqueID
}

func (e *Zigbee2MQTTDevice) getFriendlyName() string {
	var friendlyName string

	if e.IsDevice {
		if e.actionPrefix != "" {
			friendlyName = fmt.Sprintf("%s Button %s", e.z2MDevice.FriendlyName, e.actionPrefix)
		} else {
			friendlyName = e.z2MDevice.FriendlyName
		}
	}

	if e.IsGroup {
		friendlyName = e.z2MGroup.FriendlyName
	}

	return friendlyName
}

func (e *Zigbee2MQTTDevice) StartDiscovery(vdcdClient *vdcdapi.Client, mqttClient mqtt.Client) {
	e.mqttClient = mqttClient
	e.vdcdClient = vdcdClient

	if e.mqttProxy == nil {
		e.mqttProxy = new(MQTTProxy)
		e.mqttProxy.mqttClient = e.mqttClient
	}

	log.Info(("Starting Zigbee2MQTT Device discovery"))
	e.subscribeMqttTopic("zigbee2mqtt/bridge/#", e.mqttDiscoverCallback())
}

func (e *Zigbee2MQTTDevice) mqttDiscoverCallback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		if strings.Contains(msg.Topic(), "devices") {
			log.WithFields(log.Fields{
				"Topic": msg.Topic(),
			}).Debugf("MQTT Mesage for Zigbee2MQTT Device Discovery: %s", string(msg.Payload()))

			var z2Mdevices []Z2MDevice
			if err := json.Unmarshal([]byte(msg.Payload()), &z2Mdevices); err != nil {
				log.WithError(err).Error("Failed to Unmarshal Z2MDevice")
			}

			for _, z2mdevice := range z2Mdevices {
				log.WithFields(log.Fields{
					"Friendly Name": z2mdevice.FriendlyName,
				}).Info("Discovered Z2M device")

				rawDevice, _ := json.Marshal(z2mdevice)
				log.WithFields(log.Fields{
					"Friendly Name": z2mdevice.FriendlyName,
				}).Debug("Raw Device: ", string(rawDevice))

				switch z2mdevice.Definition.Model {
				case "WXCJKG13LM":
					// Opple wireless switch (triple band)
					for i := 1; i < 6; i++ {
						e.CreateButtonDevice(z2mdevice, i, fmt.Sprintf("button_%d", i+1))
					}
				case "WXCJKG11LM":
					// Opple wireless switch (single band)
					for i := 1; i < 2; i++ {
						e.CreateButtonDevice(z2mdevice, i, fmt.Sprintf("button_%d", i+1))
					}
				case "WXCJKG12LM":
					// Opple wireless switch (double band)
					for i := 1; i < 4; i++ {
						e.CreateButtonDevice(z2mdevice, i, fmt.Sprintf("button_%d", i+1))
					}
				case "E1524/E1810":
					// TRADFRI remote control
					e.CreateButtonDevice(z2mdevice, 0, "toggle")
					e.CreateButtonDevice(z2mdevice, 1, "brightness_up")
					e.CreateButtonDevice(z2mdevice, 2, "brightness_down")
					e.CreateButtonDevice(z2mdevice, 3, "arrow_left")
					e.CreateButtonDevice(z2mdevice, 4, "arrow_right")

				case "LED1623G12":
					// TRADFRI bulb E27, white, globe, opal, 1000 lm
				case "LED2101G4":
					// TRADFRI bulb E12/E14, white spectrum, globe, opal, 450/470 lm
				case "LED1650R5":
					// TRADFRI bulb GU10, white, 400 lm
					//e.CreateLightDevice(device)
				}
			}
		}

		if strings.Contains(msg.Topic(), "groups") {
			log.WithFields(log.Fields{
				"Topic": msg.Topic(),
			}).Debugf("MQTT Mesage for Zigbee2MQTT Device Discovery: %s\n", string(msg.Payload()))

			var groups []Z2MGroup
			if err := json.Unmarshal([]byte(msg.Payload()), &groups); err != nil {
				log.WithError(err).Error("Failed to unmarshall Z2MGroup")
			}

			for _, group := range groups {
				log.WithFields(log.Fields{
					"Friendly Name": group.FriendlyName,
				}).Info("Found new Zigbee2MQTT Group")

				zigbee2mqttdevice := new(Zigbee2MQTTDevice)
				zigbee2mqttdevice.z2MGroup = group
				zigbee2mqttdevice.IsGroup = true

				zigbee2mqttdevice.Topic = group.FriendlyName

				_, notfounderr := e.vdcdClient.GetDeviceByUniqueId(group.FriendlyName)
				if notfounderr != nil {
					log.WithField("FriendlyName", group.FriendlyName).Debug("Zigbee2MQTT Group not found in vcdc")
					zigbee2mqttdevice.NewZigbee2MQTT(e.vdcdClient, e.mqttClient, nil)
				}
			}
		}

	}

	return f
}

func (e *Zigbee2MQTTDevice) configureCallbacks() {

	log.WithFields(log.Fields{
		"UniqueID":      e.getUniqueId(),
		"Friendly Name": e.getFriendlyName(),
	}).Debug("Subscribe to MQTT topic")

	// Add callback
	topic := fmt.Sprintf("zigbee2mqtt/%s", e.Topic)
	e.mqttProxy.subscribeMqttTopic(topic, e.mqttCallback())
	topicAction := fmt.Sprintf("zigbee2mqtt/%s/action", e.Topic)
	e.mqttProxy.subscribeMqttTopic(topicAction, e.mqttActionCallback())

}

// MQTT Callback from zigbee2mqtt device
// This updates the dss channel on the linked origin vdcd-brige Device
func (e *Zigbee2MQTTDevice) mqttCallback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		log.WithFields(log.Fields{
			"FriendlyName": e.FriendlyName,
			"Topic":        msg.Topic(),
		}).Debugf("Zigbee2MQTT Message %s", string(msg.Payload()))

		var deviceData Z2MDeviceData
		if err := json.Unmarshal([]byte(msg.Payload()), &deviceData); err != nil {
			log.WithError(err).Info("Failed to unmarshall device Data received from MQTT")
		}

		if deviceData.Brightness != nil {
			b := *deviceData.Brightness / 254 * 100
			e.originDevice.UpdateValue(float32(b), "brightness", vdcdapi.BrightnessType)
		}

		if deviceData.State != nil {
			if *deviceData.State == "ON" {
				e.originDevice.UpdateValue(100, "basic_switch", vdcdapi.UndefinedType)
			} else {
				e.originDevice.UpdateValue(0, "basic_switch", vdcdapi.UndefinedType)
			}
		}

		if deviceData.ColorTemp != nil {
			e.originDevice.UpdateValue(float32(*deviceData.ColorTemp), "colortemp", vdcdapi.ColorTemperatureType)
		}

	}

	return f
}

// MQTT Callback from zigbee2mqtt device
// This is called when a devices emits an action
func (e *Zigbee2MQTTDevice) mqttActionCallback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		log.WithFields(log.Fields{
			"FriendlyName": e.FriendlyName,
			"Topic":        msg.Topic(),
		}).Debugf("Zigbee2MQTT Action Message %s", string(msg.Payload()))

		action := strings.Replace(string(msg.Payload()), fmt.Sprintf("%s_", e.actionPrefix), "", -1)

		// Check if the message is for this device, because they share the same mqtt topic
		if strings.Contains(action, e.actionPrefix) {
			log.WithFields(log.Fields{
				"FriendlyName": e.FriendlyName,
				"Topic":        msg.Topic(),
			}).Debugf("Action Event for this Device: %s", action)

			switch action {
			case "hold":
				e.vdcdClient.SendButtonMessage(1, e.originDevice.Tag, 0)
			case "release":
				e.vdcdClient.SendButtonMessage(0, e.originDevice.Tag, 0)
			case "single":
			case "click":
				e.vdcdClient.SendButtonRawMessage(vdcdapi.CT_TIP_1X, e.originDevice.Tag, 0)
			case "double":
				e.vdcdClient.SendButtonRawMessage(vdcdapi.CT_TIP_2X, e.originDevice.Tag, 0)
			case "triple":
				e.vdcdClient.SendButtonRawMessage(vdcdapi.CT_TIP_3X, e.originDevice.Tag, 0)
			default:
				e.vdcdClient.SendButtonRawMessage(vdcdapi.CT_TIP_1X, e.originDevice.Tag, 0)

			}

		}

	}

	return f
}

func (e *Zigbee2MQTTDevice) vcdcChannelCallback() func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {

	var f func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) = func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {
		log.WithField("Device", device.UniqueID).Debug("vcdcCallBack called")
		e.SetValue(message.Value, message.ChannelName, message.ChannelType)
	}

	return f
}

// Apply update from dss to shelly
func (e *Zigbee2MQTTDevice) SetValue(value float32, channelName string, channelType vdcdapi.ChannelTypeType) {

	log.WithFields(log.Fields{
		"FriendlyName": e.FriendlyName,
		"ChannelName":  channelName}).Infof("Set Value to %f \n", value)

	// Also sync the state with originDevice
	e.originDevice.SetValue(value, channelName)

	switch channelName {
	case "basic_switch":
		if value == 100 {
			e.TurnOn()
		} else {
			e.TurnOff()
		}

	case "brightness":
		e.SetBrightness(value)

	case "colortemp":
		e.SetColorTemp(value)

	}

}

func (e *Zigbee2MQTTDevice) TurnOn() {
	e.publishMqttCommand("zigbee2mqtt/"+e.Topic+"/set/state", "on")
}

func (e *Zigbee2MQTTDevice) TurnOff() {
	e.publishMqttCommand("zigbee2mqtt/"+e.Topic+"/set/state", "off")
}

func (e *Zigbee2MQTTDevice) SetBrightness(brightness float32) {
	b := brightness / 100 * 254
	e.publishMqttCommand("zigbee2mqtt/"+e.Topic+"/set/brightness", b)
}

func (e *Zigbee2MQTTDevice) SetColorTemp(ct float32) {
	e.publishMqttCommand("zigbee2mqtt/"+e.Topic+"/set/color_temp", ct)
}

func (p *MQTTProxy) subscribeMqttTopic(topic string, callback mqtt.MessageHandler) {

	strippedTopic := strings.Replace(topic, "/#", "", -1)

	if p.receivers == nil {
		p.receivers = make(map[string][]mqtt.MessageHandler)
	}

	if len(p.receivers[strippedTopic]) == 0 {
		// its the first receiver
		// Subscribe

		log.WithFields(log.Fields{
			"Topic": topic,
		}).Debug("MQTT Proxy Subscribe to topic")
		if token := p.mqttClient.Subscribe(topic, 0, p.callback()); token.Wait() && token.Error() != nil {
			log.Error("MQTT Proxy subscribe failed: ", token.Error())
		}

	}

	log.WithFields(log.Fields{
		"Topic": topic,
	}).Debug("Append new Callback Receiver for MQTT topic")
	p.receivers[strippedTopic] = append(p.receivers[strippedTopic], callback)

}

func (p *MQTTProxy) callback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		log.WithFields(log.Fields{
			"Topic":      msg.Topic(),
			"# Receiver": len(p.receivers[msg.Topic()]),
		}).Debug("MQTT Message received on MQTT Proxy -> Forward to all Callbacks")

		// Forward message to all receiver suvscribed to this topic
		for _, receiver := range p.receivers[msg.Topic()] {
			receiver(client, msg)
		}
	}
	return f

}
