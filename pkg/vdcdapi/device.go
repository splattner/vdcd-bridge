package vdcdapi

import (
	log "github.com/sirupsen/logrus"
)

func (e *Device) NewDevice(client *Client, uniqueID string) {
	e.UniqueID = uniqueID
	e.Tag = uniqueID
	e.Output = BasicOutput

	e.client = client
	e.ModelName = e.client.modelName
	e.VendorName = e.client.vendorName
}

func (e *Device) NewLightDevice(client *Client, uniqueID string, dimmable bool) {
	e.NewDevice(client, uniqueID)

	if dimmable {
		e.Output = LightOutput
	}

	e.Group = YellowLightGroup
	e.ColorClass = YellowColorClassT
}

func (e *Device) SetName(name string) {
	e.Name = name
}

func (e *Device) SetTag(tag string) {
	e.Tag = tag
}

func (e *Device) AddButton(button Button) {
	e.Buttons = append(e.Buttons, button)
}

func (e *Device) AddSensor(sensor Sensor) {
	e.Sensors = append(e.Sensors, sensor)
}

func (e *Device) AddInput(input Input) {
	e.Inputs = append(e.Inputs, input)
}

// Update value from smartdevice to vdcd-bridge Device and send update to dss
func (e *Device) UpdateValue(newValue float32, channelName string, chanelType ChannelTypeType) {

	// only update when changed
	if newValue != e.value {
		e.SetValue(newValue)
		e.client.UpdateValue(e, channelName, chanelType)
	}

}

func (e *Device) SetValue(newValue float32) {
	log.Debugf("Set value for vdcd-brige Device %s to: %f\n", e.UniqueID, newValue)
	e.value = newValue
}

func (e *Device) SetInitDone() {
	e.InitDone = true
	log.Debugf("Init for Device %s done: %t\n", e.UniqueID, e.InitDone)
}

func (e *Device) SetChannelMessageCB(cb func(message *GenericVDCDMessage, device *Device)) {
	e.channel_cb = cb
}
