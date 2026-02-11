package discovery

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/signal"
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

	IsSensor bool
	sensor   deconzsensor.Sensor
	// For when there are multiple buttons on one sensor
	// sensorButtonId is the identifier to get the correct device
	sensorButtonId int

	// Array with all lights, groups, sensors
	allDeconzDevices []DeconzDevice
	websocketStarted bool

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

func (e *DeconzDevice) hasDeconzDevice(resource string, id int) bool {
	for _, device := range e.allDeconzDevices {
		switch resource {
		case "lights":
			if device.IsLight && device.light.ID == id {
				return true
			}
		case "groups":
			if device.IsGroup && device.group.ID == id {
				return true
			}
		case "sensors":
			if device.IsSensor && device.sensor.ID == id {
				return true
			}
		}
	}
	return false
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

func (e *DeconzDevice) NewDeconzDevice(vdcdClient *vdcdapi.Client, deconzHost string, deconzPort int, deconzWebSocketPort int, deconzAPI string) *vdcdapi.Device {
	e.vdcdClient = vdcdClient

	e.deconzHost = deconzHost
	e.deconzPort = deconzPort
	e.deconzWebSocketPort = deconzWebSocketPort
	e.deconzAPI = deconzAPI

	var device *vdcdapi.Device

	if e.IsLight {
		device = e.NewDeconzLightDevice()
	}

	if e.IsGroup {
		device = e.NewDeconzGroupDevice()
	}

	if e.IsSensor {
		device = e.NewDeconzSensorDevice()
	}

	return device
}

func (e *DeconzDevice) StartDiscovery(vdcdClient *vdcdapi.Client, deconzHost string, deconzPort int, deconcWebSockerPort int, deconzAPI string, enableGroups bool) {
	e.vdcdClient = vdcdClient

	e.deconzHost = deconzHost
	e.deconzPort = deconzPort
	e.deconzWebSocketPort = deconcWebSockerPort
	e.deconzAPI = deconzAPI

	host := fmt.Sprintf("%s:%d", deconzHost, deconzPort)

	log.Infof("Starting Deconz device discovery on host %s\n", host)

	// Lights
	dl := deconzlight.New(host, e.deconzAPI)
	allLights, _ := dl.GetAllLights()
	for _, light := range allLights {
		e.lightsDiscovery(light)
	}

	// Groups
	if enableGroups {
		dg := deconzgroup.New(host, e.deconzAPI)
		allGroups, _ := dg.GetAllGroups()
		for _, group := range allGroups {
			e.groupsDiscovery(group)
		}
	}

	// Sensors
	ds := deconzsensor.New(host, e.deconzAPI)
	allSensors, _ := ds.GetAllSensors()
	for _, sensor := range allSensors {
		e.sensorDiscovery(sensor)
	}

	// WebSocket Handling for all Devices
	// no need for every device to open its own websocket connection
	if !e.websocketStarted {
		e.websocketStarted = true
		go e.websocketLoop()
	}

	log.Debugf("Deconz, Device Discovery finished\n")
}

func (e *DeconzDevice) websocketLoop() {

	log.Debugln("Deconz, Starting Deconz Websocket Loop")
	e.done = make(chan interface{})    // Channel to indicate that the receiverHandler is done
	e.interrupt = make(chan os.Signal) // Channel to listen for interrupt signal to terminate gracefully

	signal.Notify(e.interrupt, os.Interrupt) // Notify the interrupt channel for SIGINT

	socketUrl := fmt.Sprintf("ws://%s:%d", e.deconzHost, e.deconzWebSocketPort)
	log.Debugf("Deconz, Trying to connect to Deconz Websocket %s\n", socketUrl)
	conn, _, err := websocket.DefaultDialer.Dial(socketUrl, nil)
	if err != nil {
		log.Fatal("Deconz, Error connecting to Websocket Server:", err)
	}
	log.Debugln("Deconz, Connected to Deconz websocket")

	defer func() {
		if err := conn.Close(); err != nil {
			log.WithError(err).Warn("Deconz, Error closing websocket connection")
		}
	}()

	go e.websocketReceiveHandler(conn)

	// Our main loop for the client
	// We send our relevant packets here
	log.Debugln("Deconz, Starting Deconz Websocket client main loop")
	for { // nolint:all
		select {
		case <-e.interrupt:
			// We received a SIGINT (Ctrl + C). Terminate gracefully...
			log.Println("Deconz, Received SIGINT interrupt signal. Closing all pending connections")

			// Close our websocket connection
			err := conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("Deconz, Error during closing websocket:", err)
				return
			}

			select {
			case <-e.done:
				log.Println("Deconz, Receiver Channel Closed!")
			case <-time.After(time.Duration(1) * time.Second):
				log.Println("Deconz, Timeout in closing receiving channel.")
			}
			return
		}
	}
}

func (e *DeconzDevice) websocketReceiveHandler(connection *websocket.Conn) {

	log.Debugln("Deconz, Starting Deconz Websocket receive handler")

	defer close(e.done)
	for {
		_, msg, err := connection.ReadMessage()
		if err != nil {
			log.Println("Deconz, Error in Deconz Websocket Message receive:", err)
			return
		}

		log.Debugf("Deconz, Received Deconz Websocket Message. Raw Message: %s\n", msg)

		var message DeconzWebSocketMessage
		err = json.Unmarshal(msg, &message)

		if err != nil {
			log.WithError(err).Error("Unmarshal to DeconzWebSocketMessage failed")
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

			for _, l := range e.allDeconzDevices {
				if l.IsGroup {
					if fmt.Sprint(l.group.ID) == message.ID {
						log.Infof("Deconz Websocket changed event for group %s\n", l.group.Name)
						l.groupStateChangedCallback(&message.State)
						break
					}

				}

			}

		}

		// Handling sensor Resources
		if message.Type == "event" && message.Resource == "sensors" && message.Event == "changed" {

			for _, l := range e.allDeconzDevices {
				if l.IsSensor {
					if fmt.Sprint(l.sensor.ID) == message.ID {
						// Send to all devices which handles this sensor
						log.Infof("Deconz, Websocket changed event for sensor %s\n", l.sensor.Name)
						l.sensorWebsocketCallback(&message.State)
					}
				}
			}
		}
	}
}

// Apply update from dss to deconz device
func (e *DeconzDevice) SetValue(value float32, channelName string, channelType vdcdapi.ChannelTypeType) {

	log.Infof("Deconz, Set Value for Deconz Device %s to %f on Channel '%s' \n", e.light.Name, value, channelName)

	// Also sync the state with originDevice
	e.originDevice.SetValue(value, channelName)

	switch channelName {

	case "basic_switch", "brightness":
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

	f := func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {
		log.Debugf("Deconz, vcdcCallBack called for Device %s\n", device.UniqueID)
		e.SetValue(message.Value, message.ChannelName, message.ChannelType)
	}

	return f
}

func (e *DeconzDevice) TurnOn() {

	if e.IsLight {
		e.light.State.SetOn(true)
	}

	if e.IsGroup {
		e.group.Action.SetOn(true)
	}

	e.setState()
}

func (e *DeconzDevice) TurnOff() {

	if e.IsLight {
		e.light.State.SetOn(false)
	}

	if e.IsGroup {
		e.group.Action.SetOn(false)
	}

	e.setState()
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

	}

	if e.IsGroup {
		if brightness == 0 {
			e.group.Action.SetOn(false)
		} else {
			e.group.Action.SetOn(true)
		}

		bri_converted := uint8(math.Round(float64(brightness) / 100 * 255))
		e.group.Action.Bri = &bri_converted
	}

	e.setState()
}

func (e *DeconzDevice) SetColorTemp(ct float32) {

	converted := uint16(ct)

	if e.IsLight {
		e.light.State.CT = &converted
	}

	if e.IsGroup {
		e.group.Action.CT = &converted
	}

	e.setState()
}

func (e *DeconzDevice) SetHue(hue float32) {

	converted := uint16(hue)
	if e.IsLight {
		e.light.State.Hue = &converted
	}

	if e.IsGroup {
		e.group.Action.Hue = &converted
	}

	e.setState()
}

func (e *DeconzDevice) SetSaturation(saturation float32) {

	converted := uint8(saturation)

	if e.IsLight {
		e.light.State.Sat = &converted
	}

	if e.IsGroup {
		e.group.Action.Sat = &converted
	}

	e.setState()
}

func (e *DeconzDevice) setState() {

	if e.IsLight {
		log.Debug("Set Light state for")
		//e.setLightState()
	}

	if e.IsGroup {
		log.Debug("Set Group state for")
		//e.setGroupState()
	}
}
