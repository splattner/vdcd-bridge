package vdcdapi

import (
	"errors"

	log "github.com/sirupsen/logrus"
)

func (e *Device) NewDevice(client *Client, uniqueID string) {
	e.UniqueID = uniqueID
	e.Tag = uniqueID
	e.Output = BasicOutput

	e.client = client
	e.ModelName = e.client.modelName
	e.VendorName = e.client.vendorName

	basicChannel := new(Channel)
	basicChannel.ChannelName = "basic_switch"
	basicChannel.ChannelType = UndefinedType

	e.AddChannel(*basicChannel)
}

func (e *Device) NewLightDevice(client *Client, uniqueID string, dimmable bool) {
	e.NewDevice(client, uniqueID)

	if dimmable {
		e.Output = LightOutput
		brightnessChannel := new(Channel)
		brightnessChannel.ChannelName = "brightness"
		brightnessChannel.ChannelType = BrightnessType

		e.AddChannel(*brightnessChannel)
	}

	e.Group = YellowLightGroup
	e.ColorClass = YellowColorClassT
}

func (e *Device) NewColorLightDevice(client *Client, uniqueID string) {
	e.NewDevice(client, uniqueID)

	e.Output = ColorLightOutput

	brightnessChannel := new(Channel)
	brightnessChannel.ChannelName = "brightness"
	brightnessChannel.ChannelType = BrightnessType

	hueChannel := new(Channel)
	hueChannel.ChannelName = "hue"
	hueChannel.ChannelType = HueType

	saturationChannel := new(Channel)
	saturationChannel.ChannelName = "saturation"
	saturationChannel.ChannelType = SaturationType

	colorTempChannel := new(Channel)
	colorTempChannel.ChannelName = "colortemp"
	colorTempChannel.ChannelType = ColorTemperatureType

	e.AddChannel(*brightnessChannel)
	e.AddChannel(*hueChannel)
	e.AddChannel(*saturationChannel)
	e.AddChannel(*colorTempChannel)

	e.Group = YellowLightGroup
	e.ColorClass = YellowColorClassT
}

func (e *Device) NewCTLightDevice(client *Client, uniqueID string) {
	e.NewDevice(client, uniqueID)

	e.Output = CtLightOutput

	brightnessChannel := new(Channel)
	brightnessChannel.ChannelName = "brightness"
	brightnessChannel.ChannelType = BrightnessType

	colorTempChannel := new(Channel)
	colorTempChannel.ChannelName = "colortemp"
	colorTempChannel.ChannelType = ColorTemperatureType

	e.AddChannel(*brightnessChannel)
	e.AddChannel(*colorTempChannel)

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
func (e *Device) UpdateValue(newValue float32, channelName string, channelType ChannelTypeType) {
	log.Debugf("Value update from smartdevice to vdcd-bridge Device %s -> send update to dss  Value: %f,  ChannelName: %s\n", e.UniqueID, newValue, channelName)

	for i := 0; i < len(e.Channels); i++ {
		if e.Channels[i].ChannelName == channelName {
			if e.Channels[i].Value != newValue {
				// only update when changed
				e.SetValue(newValue, channelName)
				e.client.UpdateValue(e, channelName, channelType)
				break
			}
		}
	}

}

func (e *Device) UpdateSensorValue(newValue float32, sensorId string) {

	for i := 0; i < len(e.Sensors); i++ {
		if e.Sensors[i].Id == sensorId {
			e.client.SendSensorMessage(newValue, e.Tag, sensorId, i)
			break
		}
	}

}

func (e *Device) SetValue(newValue float32, channelName string) {
	log.Debugf("Set value for vdcd-brige Device %s to: %f on ChannelName: %s\n", e.UniqueID, newValue, channelName)
	for i := 0; i < len(e.Channels); i++ {
		if e.Channels[i].ChannelName == channelName {
			e.Channels[i].Value = newValue
			break
		}
	}
}

func (e *Device) GetValue(channelName string) (float32, error) {
	for i := 0; i < len(e.Channels); i++ {
		if e.Channels[i].ChannelName == channelName {
			return e.Channels[i].Value, nil
		}
	}

	return float32(0), errors.New(("Channel for Device not found"))
}

func (e *Device) SetInitDone() {
	e.InitDone = true
	log.Debugf("Init for Device %s done: %t\n", e.UniqueID, e.InitDone)
}

func (e *Device) SetChannelMessageCB(cb func(message *GenericVDCDMessage, device *Device)) {
	e.channel_cb = cb
}

func (e *Device) AddChannel(channel Channel) {
	e.Channels = append(e.Channels, channel)
}
