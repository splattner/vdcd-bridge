package discovery

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"

	"github.com/splattner/vdcd-bridge/pkg/vdcdapi"

	deconzgroup "github.com/jurgen-kluft/go-conbee/groups"
	deconzlight "github.com/jurgen-kluft/go-conbee/lights"
	deconzsensor "github.com/jurgen-kluft/go-conbee/sensors"
)

type DeconzDevice struct {
	GenericDevice

	deconzHost          string
	deconzPort          int
	deconzWebSocketPort int
	deconzAPI           string

	IsLight bool
	light   deconzlight.Light

	IsGroup bool
	group   deconzgroup.Group

	IsSensor       bool
	sensor         deconzsensor.Sensor
	sensorButtonID int

	allDeconzDevices []DeconzDevice

	done      chan interface{}
	interrupt chan os.Signal
}

type DeconzWebSocketMessage struct {
	Type       string               `json:"t,omitempty"`
	Event      string               `json:"e,omitempty"`
	Resource   string               `json:"r,omitempty"`
	ID         string               `json:"id,omitempty"`
	UniqueID   string               `json:"uniqueid,omitempty"`
	GroupID    string               `json:"gid,omitempty"`
	SceneID    string               `json:"scid,omitempty"`
	Name       string               `json:"name,omitempty"`
	Attributes DeconzLightAttribute `json:"attr,omitempty"`
	State      DeconzState          `json:"state,omitempty"`
}

type DeconzLightAttribute struct {
	Id                string `json:"id,omitempty"`
	LastAnnounced     string `json:"lastannounced,omitempty"`
	LastSeen          string `json:"lastseen,omitempty"`
	ManufacturerName  string `json:"manufacturername,omitempty"`
	ModelId           string `json:"modelid,omitempty"`
	Name              string `json:"name,omitempty"`
	SWVersion         string `json:"swversion,omitempty"`
	Type              string `json:"type,omitempty"`
	UniqueID          string `json:"uniqueid,omitempty"`
	ColorCapabilities int    `json:"colorcapabilities,omitempty"`
	Ctmax             int    `json:"ctmax,omitempty"`
	Ctmin             int    `json:"ctmin,omitempty"`
}

type DeconzState struct {

	// Light & Group
	On     *bool     `json:"on,omitempty"`     //
	Hue    *uint16   `json:"hue,omitempty"`    //
	Effect string    `json:"effect,omitempty"` //
	Bri    *uint8    `json:"bri,omitempty"`    // min = 1, max = 254
	Sat    *uint8    `json:"sat,omitempty"`    //
	CT     *uint16   `json:"ct,omitempty"`     // min = 154, max = 500
	XY     []float32 `json:"xy,omitempty"`
	Alert  string    `json:"alert,omitempty"`

	// Light
	Reachable      *bool   `json:"reachable,omitempty"`
	ColorMode      string  `json:"colormode,omitempty"`
	ColorLoopSpeed *uint8  `json:"colorloopspeed,omitempty"`
	TransitionTime *uint16 `json:"transitiontime,omitempty"`

	// Group
	AllOn bool `json:"all_on,omitempty"`
	AnyOn bool `json:"any_on,omitempty"`

	// Sensor
	ButtonEvent int `json:"buttonevent,omitempty"`
}

type ButtonEvent int

const (
	Hold ButtonEvent = iota + 1
	ShortRelease
	DoublePress
	TreeplePress
	LongRelease
)

func (e *DeconzDevice) NewDeconzDevice(vdcdClient *vdcdapi.Client, deconzHost string, deconzPort int, deconzWebSocketPort int, deconzAPI string) *vdcdapi.Device {
	e.vdcdClient = vdcdClient

	e.deconzHost = deconzHost
	e.deconzPort = deconzPort
	e.deconzWebSocketPort = deconzWebSocketPort
	e.deconzAPI = deconzAPI

	device := new(vdcdapi.Device)

	device.SetChannelMessageCB(e.vcdcChannelCallback())

	if e.IsLight {

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

	}

	if e.IsGroup {

		// Group only allows for on/off -> basic switch, no dimming
		device.NewLightDevice(e.vdcdClient, fmt.Sprintf("%d", e.group.ID), false)

		device.ModelName = "Light Group"
		device.SetName(fmt.Sprintf("Group: %s", e.group.Name))

	}

	if e.IsSensor {

		log.Debugf("Adding sensor %s %s", e.sensor.Name, e.sensorButtonID)

		device.NewButtonDevice(e.vdcdClient, fmt.Sprintf("%s-%s", e.sensor.UniqueID, e.sensorButtonID))

		device.SetName(fmt.Sprintf("%s / Button %d", e.sensor.Name, e.sensorButtonID+1))

		device.ModelName = e.sensor.ModelID
		device.ModelVersion = e.sensor.SWVersion
		device.VendorName = e.sensor.ManufacturerName

		button := new(vdcdapi.Button)
		button.Id = "button" //fmt.Sprint(e.sensorButtonID)
		button.ButtonId = e.sensorButtonID
		button.ButtonType = vdcdapi.SingleButton
		button.Group = vdcdapi.YellowLightGroup
		device.AddButton(*button)

	}

	device.ConfigUrl = fmt.Sprintf("http://%s:%d", e.deconzHost, e.deconzPort)
	device.SourceDevice = e

	e.originDevice = device
	e.vdcdClient.AddDevice(device)

	return device
}

func (e *DeconzDevice) StartDiscovery(vdcdClient *vdcdapi.Client, deconzHost string, deconzPort int, deconcWebSockerPort int, deconzAPI string, enableGroups bool) {
	e.vdcdClient = vdcdClient

	e.deconzHost = deconzHost
	e.deconzPort = deconzPort
	e.deconzWebSocketPort = deconcWebSockerPort
	e.deconzAPI = deconzAPI

	host := fmt.Sprintf("%s:%d", deconzHost, deconzPort)

	log.Infof("Starting Deconz Device discovery on host %s\n", host)

	// // Lights
	// dl := deconzlight.New(host, e.deconzAPI)

	// allLights, _ := dl.GetAllLights()
	// for _, l := range allLights {

	// 	if l.Type != "Configuration tool" { // filter this out
	// 		if *l.State.Reachable { // only available/reachable devices
	// 			deconzDevice := new(DeconzDevice)

	// 			deconzDevice.IsLight = true
	// 			deconzDevice.light = l

	// 			log.Infof("Deconz Lights discovered: Name: %s, \n", l.Name)

	// 			_, notfounderr := e.vdcdClient.GetDeviceByUniqueId(l.UniqueID)
	// 			if notfounderr != nil {
	// 				log.Debugf("Deconz Device not found in vcdc -> Adding \n")
	// 				deconzDevice.NewDeconzDevice(e.vdcdClient, e.deconzHost, e.deconzPort, e.deconzWebSocketPort, e.deconzAPI)
	// 			}

	// 			e.allDeconzDevices = append(e.allDeconzDevices, *deconzDevice)
	// 		}
	// 	}

	// }

	// Groups
	if enableGroups {
		dg := deconzgroup.New(host, e.deconzAPI)

		allGroups, _ := dg.GetAllGroups()
		for _, g := range allGroups {
			if len(g.Lights) > 0 {

				deconzDeviceGroup := new(DeconzDevice)
				deconzDeviceGroup.IsGroup = true
				deconzDeviceGroup.group = g

				log.Infof("Deconz Group discovered: Name: %s, \n", g.Name)

				_, notfounderr := e.vdcdClient.GetDeviceByUniqueId(fmt.Sprint(g.ID))
				if notfounderr != nil {
					log.Debugf("Deconz Device not found in vcdc -> Adding \n")
					deconzDeviceGroup.NewDeconzDevice(e.vdcdClient, e.deconzHost, e.deconzPort, e.deconzWebSocketPort, e.deconzAPI)
				}

				e.allDeconzDevices = append(e.allDeconzDevices, *deconzDeviceGroup)

			}
		}
	}

	// Sensors
	ds := deconzsensor.New(host, e.deconzAPI)

	allSensors, _ := ds.GetAllSensors()
	for _, sensor := range allSensors {

		if sensor.Type == "ZHASwitch" {

			log.Infof("Deconz Sensor discovered: Name: %s, Type: %s, \n", sensor.Name, sensor.Type)

			switch sensor.ModelID {
			case "lumi.remote.b286opcn01":
				sensor.Name = strings.Replace(sensor.Name, "OPPLE ", "", -1)
				e.CreateButtonDevice(&sensor, 0)
				e.CreateButtonDevice(&sensor, 1)

			case "lumi.remote.b486opcn01":
				sensor.Name = strings.Replace(sensor.Name, "OPPLE ", "", -1)
				// e.CreateButtonDevice(&sensor, 0)
				// e.CreateButtonDevice(&sensor, 1)
				// e.CreateButtonDevice(&sensor, 2)
				// e.CreateButtonDevice(&sensor, 3)

			case "lumi.remote.b686opcn01":
				sensor.Name = strings.Replace(sensor.Name, "OPPLE ", "", -1)
				// e.CreateButtonDevice(&sensor, 0)
				// e.CreateButtonDevice(&sensor, 1)
				// e.CreateButtonDevice(&sensor, 2)
				// e.CreateButtonDevice(&sensor, 3)
				// e.CreateButtonDevice(&sensor, 4)
				// e.CreateButtonDevice(&sensor, 5)

			}

		}
	}

	// WebSocket Handling for all Devices
	// no need for every device to open its own websocket connection
	log.Debugf("Call DeconZ Websocket Loop\n")
	go e.websocketLoop()

	log.Debugf("Deconz Device Discovery finished\n")
}

func (e *DeconzDevice) CreateButtonDevice(sensor *deconzsensor.Sensor, buttonID int) {
	log.Infof("Deconz, Create ButtonDevice for %s, Button: %s, \n", sensor.Name, buttonID)

	deconzDeviceSensor := new(DeconzDevice)
	deconzDeviceSensor.IsSensor = true
	deconzDeviceSensor.sensor = *sensor
	deconzDeviceSensor.sensorButtonID = buttonID

	_, notfounderr := e.vdcdClient.GetDeviceByUniqueId(fmt.Sprintf("%s-%s", sensor.UniqueID, buttonID))
	if notfounderr != nil {
		log.Debugf("Deconz Device not found in vcdc -> Adding \n")
		deconzDeviceSensor.NewDeconzDevice(e.vdcdClient, e.deconzHost, e.deconzPort, e.deconzWebSocketPort, e.deconzAPI)
	}

	e.allDeconzDevices = append(e.allDeconzDevices, *deconzDeviceSensor)

}

func (e *DeconzDevice) websocketLoop() {

	log.Debugln("Starting Deconz Websocket Loop")
	e.done = make(chan interface{})    // Channel to indicate that the receiverHandler is done
	e.interrupt = make(chan os.Signal) // Channel to listen for interrupt signal to terminate gracefully

	signal.Notify(e.interrupt, os.Interrupt) // Notify the interrupt channel for SIGINT

	socketUrl := fmt.Sprintf("ws://%s:%d", e.deconzHost, e.deconzWebSocketPort)
	log.Debugf("Trying to connect to Deconz Websocket %s\n", socketUrl)
	conn, _, err := websocket.DefaultDialer.Dial(socketUrl, nil)
	if err != nil {
		log.Fatal("Error connecting to Websocket Server:", err)
	}
	log.Debugln("Connected to Deconz websocket")

	defer conn.Close()

	log.Debugln("Calling Deconz Websocket receive handler")
	go e.websocketReceiveHandler(conn)

	// Our main loop for the client
	// We send our relevant packets here
	log.Debugln("Starting Deconz Websocket client main loop")
	for {
		select {
		case <-time.After(time.Duration(1) * time.Millisecond * 1000):
			// Send an echo packet every second
			err := conn.WriteMessage(websocket.TextMessage, []byte("Hello from vdcd-brige!"))
			if err != nil {
				log.Println("Error during writing to websocket:", err)
				return
			}

		case <-e.interrupt:
			// We received a SIGINT (Ctrl + C). Terminate gracefully...
			log.Println("Received SIGINT interrupt signal. Closing all pending connections")

			// Close our websocket connection
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("Error during closing websocket:", err)
				return
			}

			select {
			case <-e.done:
				log.Println("Receiver Channel Closed!")
			case <-time.After(time.Duration(1) * time.Second):
				log.Println("Timeout in closing receiving channel.")
			}
			log.Debugln("Returning from Deconz Websocket Main loop")
			return
		}
	}
}

func (e *DeconzDevice) websocketReceiveHandler(connection *websocket.Conn) {

	log.Debugln("Starting Deconz Websocket receive handler")

	defer close(e.done)
	for {
		_, msg, err := connection.ReadMessage()
		if err != nil {
			log.Println("Error in Deconz Websocket Message receive:", err)
			return
		}

		log.Debugf("Received Deconz Websocket Message. Raw Message: %s\n", msg)

		var message DeconzWebSocketMessage
		err = json.Unmarshal(msg, &message)

		if err != nil {
			log.Errorf("Unmarshal to DeconzWebSocketMessage failed\n", err.Error())
			return
		}

		// Handling light Resources
		if message.Type == "event" && message.Resource == "lights" && message.Event == "changed" {
			if message.State.On != nil ||
				message.State.Hue != nil ||
				message.State.Effect != "" ||
				message.State.Bri != nil ||
				message.State.Sat != nil ||
				message.State.CT != nil ||
				message.State.Reachable != nil ||
				message.State.ColorMode != "" ||
				message.State.ColorLoopSpeed != nil {
				log.Debugln("Deconz Websocket Lights changed Event received")

				for _, l := range e.allDeconzDevices {
					if l.IsLight {
						if fmt.Sprint(l.light.ID) == message.ID {
							log.Infof("Deconz Websocket changed event for light %s\n", l.light.Name)
							l.lightStateChangedCallback(&message.State)
							break
						}

					}

				}
			}
		}

		// Handling group Resources
		if message.Type == "event" && message.Resource == "groups" && message.Event == "changed" {
			log.Debugln("Group changed Event received")

			for _, l := range e.allDeconzDevices {
				if l.IsGroup {
					if fmt.Sprint(l.group.ID) == message.ID {
						log.Infof("Deconz Websocker changed event for group %s\n", l.group.Name)
						l.groupStateChangedCallback(&message.State)
						break
					}

				}

			}

		}

		// Handling sensor Resources
		if message.Type == "event" && message.Resource == "sensors" && message.Event == "changed" {
			log.Debugln("Sensor changed Event received")

			for _, l := range e.allDeconzDevices {
				if l.IsSensor {
					if fmt.Sprint(l.sensor.ID) == message.ID {
						log.Infof("Deconz Websocker changed event for sensor %s\n", l.sensor.Name)
						l.sensorStateChangedCallback(&message.State)
						break
					}

				}

			}

		}
	}
}

// Apply update from dss to deconz device
func (e *DeconzDevice) SetValue(value float32, channelName string, channelType vdcdapi.ChannelTypeType) {

	log.Infof("Set Value for Deconz Device %s to %f on Channel '%s' \n", e.light.Name, value, channelName)

	// Also sync the state with originDevice
	e.originDevice.SetValue(value, channelName)

	switch channelName {

	case "basic_switch":
		brightness := float32(math.Round(float64(value)))
		e.SetBrightness(brightness)
	case "brightness":
		brightness := float32(math.Round(float64(value)))
		e.SetBrightness(brightness)
	case "hue":
		e.SetHue(value)
	case "saturation":
		e.SetSaturation(value)
	case "colortemp":
		e.SetColorTemp(value)

	}

}

func (e *DeconzDevice) vcdcChannelCallback() func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {

	var f func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) = func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {
		log.Debugf("vcdcCallBack called for Device %s\n", device.UniqueID)
		e.SetValue(message.Value, message.ChannelName, message.ChannelType)

	}

	return f
}

func (e *DeconzDevice) lightStateChangedCallback(state *DeconzState) {

	log.Debugf("lightStateChangedCallback called for Device '%s'. State: '%+v'\n", e.light.Name, state)

	if state.Bri != nil {
		log.Debugf("lightStateChangedCallback: set Brightness to %d\n", *state.Bri)
		bri_converted := float32(math.Round(float64(*state.Bri) / 255 * 100))
		e.originDevice.UpdateValue(float32(bri_converted), "brightness", vdcdapi.BrightnessType)
	}

	if state.CT != nil {
		log.Debugf("lightStateChangedCallback: set CT to %d\n", *state.CT)
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

func (e *DeconzDevice) groupStateChangedCallback(state *DeconzState) {

	log.Debugf("groupStateChangedCallback called for Device '%s'. State: '%+v'\n", e.light.Name, state)

	if state.AllOn {
		e.originDevice.UpdateValue(100, "basic_switch", vdcdapi.UndefinedType)
	}

	if state.AnyOn {
		e.originDevice.UpdateValue(100, "basic_switch", vdcdapi.UndefinedType)
	}

	if state.AnyOn == false {
		e.originDevice.UpdateValue(0, "basic_switch", vdcdapi.UndefinedType)
	}

}

func (e *DeconzDevice) sensorStateChangedCallback(state *DeconzState) {

	log.Debugf("sensorStateChangedCallback called for Device '%s'. State: '%+v'\n", e.sensor.Name, state)

	if state.ButtonEvent > 0 {

		// Get Button for which the event is for
		button := int(math.Round(float64(state.ButtonEvent) / 1000))
		log.Debugf("Event for Device '%s' on Button %d\n", e.sensor.Name, button)

		if e.sensor.ModelID == "lumi.remote.b286opcn01" ||
			e.sensor.ModelID == "lumi.remote.b486opcn01" ||
			e.sensor.ModelID == "lumi.remote.b686opcn01" {

			var event ButtonEvent
			event = ButtonEvent(state.ButtonEvent - (button * 1000))

			switch event {
			case Hold:

			case ShortRelease:
				e.vdcdClient.SendButtonMessage()

			case DoublePress:

			case TreeplePress:

			case LongRelease:

			}
		}
	}

}

func (e *DeconzDevice) TurnOn() {

	if e.IsLight {
		e.light.State.SetOn(true)
		e.setLightState()
	}

	if e.IsGroup {
		e.group.Action.SetOn(true)
		e.setGroupState()
	}
}

func (e *DeconzDevice) TurnOff() {

	if e.IsLight {
		e.light.State.SetOn(false)
		e.setLightState()
	}

	if e.IsGroup {
		e.group.Action.SetOn(false)
		e.setGroupState()
	}
}

func (e *DeconzDevice) SetBrightness(brightness float32) {

	if e.IsLight {
		if brightness == 0 {
			e.light.State.SetOn(false)
		} else {
			e.light.State.SetOn(true)
		}

		bri_converted := uint8(math.Round(float64(brightness) / 100 * 255))
		e.light.State.Bri = &bri_converted

		e.setLightState()
	}

	if e.IsGroup {
		if brightness == 0 {
			e.group.Action.SetOn(false)
		} else {
			e.group.Action.SetOn(true)
		}

		bri_converted := uint8(math.Round(float64(brightness) / 100 * 255))
		e.group.Action.Bri = &bri_converted

		e.setGroupState()
	}
}

func (e *DeconzDevice) SetColorTemp(ct float32) {

	converted := uint16(ct)

	if e.IsLight {
		e.light.State.CT = &converted
		e.setLightState()
	}

	if e.IsGroup {
		e.group.Action.CT = &converted
		e.setGroupState()
	}
}

func (e *DeconzDevice) SetHue(hue float32) {

	converted := uint16(hue)
	if e.IsLight {
		e.light.State.Hue = &converted
		e.setLightState()
	}

	if e.IsGroup {
		e.group.Action.Hue = &converted
		e.setGroupState()
	}
}

func (e *DeconzDevice) SetSaturation(saturation float32) {

	converted := uint8(saturation)

	if e.IsLight {
		e.light.State.Sat = &converted
		e.setLightState()
	}

	if e.IsGroup {
		e.group.Action.Sat = &converted
		e.setGroupState()
	}

}

func (e *DeconzDevice) setLightState() {

	state := strings.Replace(e.light.State.String(), "\n", ",", -1)
	state = strings.Replace(state, " ", "", -1)

	log.Infof("Deconz call SetLightState with state (%s) for Light with id %d\n", state, e.light.ID)

	conbeehost := fmt.Sprintf("%s:%d", e.deconzHost, e.deconzPort)
	ll := deconzlight.New(conbeehost, e.deconzAPI)
	_, err := ll.SetLightState(e.light.ID, &e.light.State)
	if err != nil {
		log.Debugln("SetLightState Error", err)
	}
}

func (e *DeconzDevice) setGroupState() {

	state := strings.Replace(e.group.Action.String(), "\n", ",", -1)
	state = strings.Replace(state, " ", "", -1)

	log.Infof("Deconz call SetGroupState with state (%s) for Light with id %d\n", state, e.group.ID)

	conbeehost := fmt.Sprintf("%s:%d", e.deconzHost, e.deconzPort)
	ll := deconzgroup.New(conbeehost, e.deconzAPI)
	_, err := ll.SetGroupState(e.light.ID, e.group.Action)
	if err != nil {
		log.Debugln("SetGroupState Error", err)
	}
}
