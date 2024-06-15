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

func (e *Zigbee2MQTTDevice) NewZigbee2MQTT(vdcdClient *vdcdapi.Client, mqttClient mqtt.Client) *vdcdapi.Device {
	e.vdcdClient = vdcdClient
	e.mqttClient = mqttClient

	device := new(vdcdapi.Device)
	device.SetChannelMessageCB(e.vcdcChannelCallback())
	device.SourceDevice = e
	e.originDevice = device

	if e.IsDevice {
		e.FriendlyName = e.z2MDevice.FriendlyName

		var isLight, hasState, hasBrighness, hasColorTemp bool

		for _, feature := range e.z2MDevice.Definition.Exposes {
			switch feature.Type {
			case "light":
				isLight = true
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

		if isLight {
			if hasState && hasBrighness && !hasColorTemp {
				device.NewLightDevice(e.vdcdClient, e.z2MDevice.IEEEAddress, true)
			}

			if hasState && hasBrighness && hasColorTemp {
				device.NewCTLightDevice(e.vdcdClient, e.z2MDevice.IEEEAddress)
			}

		} else {
			// only add lights for the moment
			return nil
		}

		device.SetName(e.z2MDevice.FriendlyName)
		device.ModelName = e.z2MDevice.ModelID
		//device.ModelVersion = e.z2MDevice.SoftwareBuildID
	}

	if e.IsGroup {
		e.FriendlyName = e.z2MGroup.FriendlyName

		device.NewLightDevice(e.vdcdClient, e.z2MGroup.FriendlyName, false)
		device.SetName(e.z2MGroup.FriendlyName)

		// only add lights for the moment
		return nil

	}

	log.WithFields(log.Fields{
		"FriendlyName": e.FriendlyName}).Debug("Adding Zigbee2MQTT Device or Groupto vcdc\n")
	e.configureCallbacks()
	e.vdcdClient.AddDevice(device)

	return device

}

func (e *Zigbee2MQTTDevice) StartDiscovery(vdcdClient *vdcdapi.Client, mqttClient mqtt.Client) {
	e.mqttClient = mqttClient
	e.vdcdClient = vdcdClient

	log.Info(("Starting Zigbee2MQTT Device discovery"))
	e.subscribeMqttTopic("zigbee2mqtt/bridge/#", e.mqttDiscoverCallback())
}

func (e *Zigbee2MQTTDevice) mqttDiscoverCallback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		if strings.Contains(msg.Topic(), "devices") {
			log.WithFields(log.Fields{
				"Topic": msg.Topic(),
			}).Debugf("MQTT Mesage for Zigbee2MQTT Device Discovery: %s\n", string(msg.Payload()))

			var devices []Z2MDevice
			if err := json.Unmarshal([]byte(msg.Payload()), &devices); err != nil {
				log.WithError(err).Error("Failed to Unmarshal Z2MDevice")
			}

			for _, device := range devices {
				log.WithFields(log.Fields{
					"Friendly Name": device.FriendlyName,
				}).Info("Found new Zigbee2MQTT device")

				zigbee2mqttdevice := new(Zigbee2MQTTDevice)
				zigbee2mqttdevice.z2MDevice = device
				zigbee2mqttdevice.IsDevice = true

				zigbee2mqttdevice.Topic = device.IEEEAddress
				if device.FriendlyName != "" {
					zigbee2mqttdevice.Topic = device.FriendlyName
				}

				_, notfounderr := e.vdcdClient.GetDeviceByUniqueId(device.IEEEAddress)
				if notfounderr != nil {
					log.WithField("FriendlyName", device.FriendlyName).Debug("Zigbee2MQTT Device not found in vcdc")
					zigbee2mqttdevice.NewZigbee2MQTT(e.vdcdClient, e.mqttClient)
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
					zigbee2mqttdevice.NewZigbee2MQTT(e.vdcdClient, e.mqttClient)
				}
			}
		}

	}

	return f
}

func (e *Zigbee2MQTTDevice) configureCallbacks() {

	// Add callback
	topicStat := fmt.Sprintf("zigbee2mqtt/%s/#", e.Topic)
	e.subscribeMqttTopic(topicStat, e.mqttCallback())

}

// MQTT Callback from zigbee2mqtt device
// This updates the dss channel on the linked origin vdcd-brige Device
func (e *Zigbee2MQTTDevice) mqttCallback() mqtt.MessageHandler {

	var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

		log.WithFields(log.Fields{
			"FriendlyName": e.FriendlyName,
			"Topic":        msg.Topic(),
		}).Debugf("Zigbee2MQTT Message %s", string(msg.Payload()))

		if strings.Contains(msg.Topic(), "zigbee2mqtt/"+e.Topic+"/set") {
			// filter out the set messages (they even might be from us)
			return
		}

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
