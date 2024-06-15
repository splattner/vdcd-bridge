package discovery

import (
	"fmt"
	"math"

	deconzlight "github.com/jurgen-kluft/go-conbee/lights"
	log "github.com/sirupsen/logrus"
	"github.com/splattner/vdcd-bridge/pkg/vdcdapi"
)

func (e *DeconzDevice) NewDeconzLightDevice() *vdcdapi.Device {

	device := new(vdcdapi.Device)

	device.SetChannelMessageCB(e.vcdcChannelCallback())

	if e.light.HasColor {
		if e.light.State.ColorMode == "ct" {
			device.NewCTLightDevice(e.vdcdClient, e.light.UniqueID)
		}
		if e.light.State.ColorMode == "hs" {
			device.NewColorLightDevice(e.vdcdClient, e.light.UniqueID)
		}

	} else {
		device.NewLightDevice(e.vdcdClient, e.light.UniqueID, true)
	}

	device.ModelName = e.light.ModelID
	device.ModelVersion = e.light.SWVersion
	device.SetName(e.light.Name)

	device.ConfigUrl = fmt.Sprintf("http://%s:%d", e.deconzHost, e.deconzPort)
	device.SourceDevice = e

	e.originDevice = device
	e.vdcdClient.AddDevice(device)

	return device
}

func (e *DeconzDevice) lightsDiscovery(light deconzlight.Light) {

	if light.Type != "Configuration tool" { // filter this out
		if *light.State.Reachable { // only available/reachable devices
			deconzDevice := new(DeconzDevice)

			deconzDevice.IsLight = true
			deconzDevice.light = light

			log.Infof("Deconz, Lights discovered: Name: %s, \n", light.Name)

			_, notfounderr := e.vdcdClient.GetDeviceByUniqueId(light.UniqueID)
			if notfounderr != nil {
				log.Debugf("Deconz, Device not found in vcdc -> Adding \n")
				deconzDevice.NewDeconzDevice(e.vdcdClient, e.deconzHost, e.deconzPort, e.deconzWebSocketPort, e.deconzAPI)
			}

			e.allDeconzDevices = append(e.allDeconzDevices, *deconzDevice)
		}
	}

}

func (e *DeconzDevice) lightStateChangedCallback(state *DeconzState) {

	log.Debugf("Deconz, lightStateChangedCallback called for Device '%s'. State: '%+v'\n", e.light.Name, state)

	if state.Bri != nil {
		log.Debugf("Deconz, lightStateChangedCallback: set Brightness to %d\n", *state.Bri)
		bri_converted := float32(math.Round(float64(*state.Bri) / 255 * 100))
		e.originDevice.UpdateValue(float32(bri_converted), "brightness", vdcdapi.BrightnessType)
	}

	if state.CT != nil {
		log.Debugf("Deconz, lightStateChangedCallback: set CT to %d\n", *state.CT)
		e.originDevice.UpdateValue(float32(*state.CT), "colortemp", vdcdapi.ColorTemperatureType)
	}

	// if state.Sat != nil {
	// 	log.Debugf("lightStateChangedCallback: set Saturation to %d\n", *state.Sat)
	// 	e.originDevice.UpdateValue(float32(*state.Sat), "saturation", vdcdapi.SaturationType)
	// }

	// if state.Hue != nil {
	// 	log.Debugf("lightStateChangedCallback: set Hue to %d\n", *state.Hue)
	// 	e.originDevice.UpdateValue(float32(*state.Hue), "hue", vdcdapi.HueType)
	// }

	// if !*state.On {
	// 	log.Debugf("lightStateChangedCallback: state off, set Brightness to 0\n")
	// 	e.originDevice.UpdateValue(0, "basic_switch", vdcdapi.UndefinedType)
	// }

}

// func (e *DeconzDevice) setLightState() {

// 	state := strings.Replace(e.light.State.String(), "\n", ",", -1)
// 	state = strings.Replace(state, " ", "", -1)

// 	log.Infof("Deconz, call SetLightState with state (%s) for Light with id %d\n", state, e.light.ID)

// 	conbeehost := fmt.Sprintf("%s:%d", e.deconzHost, e.deconzPort)
// 	ll := deconzlight.New(conbeehost, e.deconzAPI)
// 	_, err := ll.SetLightState(e.light.ID, &e.light.State)
// 	if err != nil {
// 		log.Debugln("Deconz, SetLightState Error", err)
// 	}
// }
