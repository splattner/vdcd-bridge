package discovery

import (
	"fmt"
	"math"
	"strings"

	deconzsensor "github.com/jurgen-kluft/go-conbee/sensors"
	log "github.com/sirupsen/logrus"
	"github.com/splattner/vdcd-bridge/pkg/vdcdapi"
)

type ButtonEvent int

// http://developer.digitalstrom.org/Architecture/ds-basics.pdf
const (
	Hold ButtonEvent = iota + 1
	ShortRelease
	LongRelease
	DoublePress
	TreeplePress
)

const (
	// milisecs
	SingleTip   int = 150
	SingleClick int = 50
)

func (e *DeconzDevice) NewDeconzSensorDevice() *vdcdapi.Device {
	log.Debugf("Deconz, Adding sensor %s for Button %d", e.sensor.Name, e.sensorButtonId)

	device := new(vdcdapi.Device)
	device.NewButtonDevice(e.vdcdClient, e.getUniqueId())

	device.SetChannelMessageCB(e.vcdcChannelCallback())

	device.SetName(e.getName())
	device.ModelName = e.sensor.ModelID
	device.ModelVersion = e.sensor.SWVersion
	device.VendorName = e.sensor.ManufacturerName

	// Add Buttons depending on the ModelId
	switch e.sensor.ModelID {
	case "lumi.remote.b286opcn01", "lumi.remote.b486opcn01", "lumi.remote.b686opcn01":
		// this is called for all buttons on these sensors
		// so it will create a individual device for each button on the sensor

		button := new(vdcdapi.Button)
		button.Id = fmt.Sprintf("button%d", e.sensorButtonId+1)
		button.ButtonId = e.sensorButtonId
		button.ButtonType = vdcdapi.SingleButton
		button.Group = vdcdapi.YellowLightGroup
		button.LocalButton = false

		device.AddButton(*button)

	}

	device.ConfigUrl = fmt.Sprintf("http://%s:%d", e.deconzHost, e.deconzPort)
	device.SourceDevice = e

	e.originDevice = device
	e.vdcdClient.AddDevice(device)

	return device
}

func (e *DeconzDevice) getUniqueId() string {
	uniqueID := fmt.Sprintf("%s-%d", e.sensor.UniqueID, e.sensorButtonId)
	return uniqueID
}

func (e *DeconzDevice) getName() string {
	name := fmt.Sprintf("%s Button %d", e.sensor.Name, e.sensorButtonId+1)
	return name
}

func (e *DeconzDevice) sensorDiscovery(sensor deconzsensor.Sensor) {

	// Call Type specifiv Discovery function
	// See https://dresden-elektronik.github.io/deconz-rest-doc/endpoints/sensors/#supported-sensor-types-and-states
	// for available types
	switch sensor.Type {
	case "ZHASwitch":
		e.ZHASwitchSensorDiscovery(sensor)
	}

}

func (e *DeconzDevice) ZHASwitchSensorDiscovery(sensor deconzsensor.Sensor) {
	log.Infof("Deconz, ZHASwitch discovered: Name: %s Model: %s\n", sensor.Name, sensor.ModelID)

	switch sensor.ModelID {
	case "lumi.remote.b286opcn01":
		sensor.Name = strings.Replace(sensor.Name, "OPPLE ", "", -1)
		e.CreateButtonDevice(sensor, 0)
		e.CreateButtonDevice(sensor, 1)

	case "lumi.remote.b486opcn01":
		sensor.Name = strings.Replace(sensor.Name, "OPPLE ", "", -1)
		e.CreateButtonDevice(sensor, 0)
		e.CreateButtonDevice(sensor, 1)
		e.CreateButtonDevice(sensor, 2)
		e.CreateButtonDevice(sensor, 3)

	case "lumi.remote.b686opcn01":
		sensor.Name = strings.Replace(sensor.Name, "OPPLE ", "", -1)
		e.CreateButtonDevice(sensor, 0)
		e.CreateButtonDevice(sensor, 1)
		e.CreateButtonDevice(sensor, 2)
		e.CreateButtonDevice(sensor, 3)
		e.CreateButtonDevice(sensor, 4)
		e.CreateButtonDevice(sensor, 5)

	}

}

func (e *DeconzDevice) CreateButtonDevice(sensor deconzsensor.Sensor, buttonId int) {
	log.Infof("Deconz, Create ButtonDevice for %s, Button: %d \n", sensor.Name, buttonId)

	deconzDeviceSensor := new(DeconzDevice)
	deconzDeviceSensor.IsSensor = true
	deconzDeviceSensor.sensor = sensor
	deconzDeviceSensor.sensorButtonId = buttonId // This is used for the uniqueId

	_, notfounderr := e.vdcdClient.GetDeviceByUniqueId(e.getUniqueId())
	if notfounderr != nil {
		log.Debugf("Deconz, Device not found in vcdc -> Adding \n")
		deconzDeviceSensor.NewDeconzDevice(e.vdcdClient, e.deconzHost, e.deconzPort, e.deconzWebSocketPort, e.deconzAPI)
	}

	e.allDeconzDevices = append(e.allDeconzDevices, *deconzDeviceSensor)

}

func (e *DeconzDevice) sensorWebsocketCallback(state *DeconzState) {

	log.Debugf("Deconz, sensorStateChangedCallback called for Device '%s'. State: '%+v'\n", e.getName(), state)

	// Only when there is a ButtonEvent
	if state.ButtonEvent > 0 {
		// Get Button for which the event is for
		// the 4 Digit is the Button Number

		button := int(math.Round(float64(state.ButtonEvent) / 1000))

		if e.sensorButtonId != button-1 {
			// Its not for this device
			// The same sensor is used by multiple devices
			// as each button of a sensor has its one device
			log.Debugf("Deconz, Event %d is not for this Device '%s' on Button %d\n", state.ButtonEvent, e.getName(), button)
			return
		}

		log.Debugf("Deconz, Event %d is for this Device '%s' on Button %d\n", state.ButtonEvent, e.getName(), button)

		switch e.sensor.ModelID {
		case "lumi.remote.b286opcn01", "lumi.remote.b486opcn01", "lumi.remote.b686opcn01":
			// the first digit is the event
			var event ButtonEvent = ButtonEvent(state.ButtonEvent - (button * 1000))
			log.Debugf("Deconz, Event %d -> %d for Device '%s' on Button %d\n", state.ButtonEvent, event, e.sensor.Name, button)

			switch event {
			case Hold:
				log.Debugf("Deconz, Event Hold for Device '%s' on Button %d\n", e.sensor.Name, button)
				e.vdcdClient.SendButtonMessage(1, e.originDevice.Tag, 0)

			case ShortRelease:
				log.Debugf("Deconz, Event ShortRelease for Device '%s' on Button %d\n", e.sensor.Name, button)
				e.vdcdClient.SendButtonMessage(float32(SingleTip), e.originDevice.Tag, 0)

			case DoublePress:
				log.Debugf("Deconz, Event DoublePress for Device '%s' on Button %d\n", e.sensor.Name, button)

			case TreeplePress:
				log.Debugf("Deconz, Event TreeplePress for Device '%s' on Button %d\n", e.sensor.Name, button)

			case LongRelease:
				log.Debugf("Deconz, Event LongRelease for Device '%s' on Button %d\n", e.sensor.Name, button)
				e.vdcdClient.SendButtonMessage(0, e.originDevice.Tag, 0)

			}
		}
	}

}
