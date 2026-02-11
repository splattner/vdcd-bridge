package vdcdapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Client struct {
	conn    net.Conn
	host    string
	port    int
	dryMode bool

	r  *bufio.Reader
	w  *bufio.Writer
	mu sync.Mutex

	dialRetry int

	devices []*Device

	modelName  string
	vendorName string

	interrupt      chan os.Signal
	receiveChannel chan string
}

func (e *Client) NewCient(host string, port int, modelName string, vendorName string, dryMode bool) {
	e.host = host
	e.port = port
	e.dialRetry = 5

	e.modelName = modelName
	e.vendorName = vendorName
	e.dryMode = dryMode
}

func (e *Client) Connect() {

	var connString = e.host + ":" + fmt.Sprint((e.port))
	var conn net.Conn
	var err error

	log.WithField("vdcd Host", connString).Info("Trying to connect to vdcd")

	for i := 0; i < e.dialRetry; i++ {

		conn, err = net.Dial("tcp", connString)

		if err != nil {
			log.WithError(err).Warn("Dial failed")
			time.Sleep(time.Second)
		} else {
			break
		}

	}

	if conn == nil {
		log.WithField("vdcd Host", connString).Error("Failed to connect to vdcd")
		os.Exit(1)
	}

	log.WithField("vdcd Host", connString).Info("Connected to vdcd")

	e.conn = conn
	e.r = bufio.NewReader(e.conn)
	e.w = bufio.NewWriter(e.conn)
}

func (e *Client) Close() {
	log.Info("Closing connection from vdcd")
	e.sendByeMessage()
	if e.conn != nil {
		if err := e.conn.Close(); err != nil {
			log.WithError(err).Warn("Failed to close connection from vdcd")
		}
	}
	log.Info("Connection from vdcd closed")
}

func (e *Client) Listen() {

	log.Info("Start listening for vdcd messages")

	e.interrupt = make(chan os.Signal)       // Channel to listen for interrupt signal to terminate gracefully
	signal.Notify(e.interrupt, os.Interrupt) // Notify the interrupt channel for SIGINT

	e.receiveChannel = make(chan string)

	go e.Receive()

	log.Debug("Start listening main loop")
	for {
		select {
		case receiveMessage := <-e.receiveChannel:
			log.Debugln("Message received from receive channel")
			var msg GenericVDCDMessage
			err := json.Unmarshal([]byte(receiveMessage), &msg)

			if err != nil {
				log.WithError(err).Error("Json Unmarshal failed")
			}

			e.processMessage(&msg)

		case <-e.interrupt:
			log.Debug("Interrupt Signal received. Returning from listening main loop")
			return

		}
	}

}

func (e *Client) ListenWithContext(ctx context.Context) {
	log.Info("Start listening for vdcd messages")

	e.receiveChannel = make(chan string)
	go e.Receive()

	log.Debug("Start listening main loop")
	for {
		select {
		case receiveMessage := <-e.receiveChannel:
			log.Debugln("Message received from receive channel")
			var msg GenericVDCDMessage
			err := json.Unmarshal([]byte(receiveMessage), &msg)
			if err != nil {
				log.WithError(err).Error("Json Unmarshal failed")
			}
			e.processMessage(&msg)
		case <-ctx.Done():
			log.Debug("Context cancelled. Returning from listening main loop")
			_ = e.conn.Close()
			return
		}
	}
}

func (e *Client) Receive() {

	log.Debug("Starting receive loop for messages from vdcd")

	for {

		log.Debug("Waiting for new vdcd message")
		line, err := e.r.ReadString('\n')

		if err != nil {
			log.WithError(err).Error("Failed to read")

			if err == io.EOF {
				// try to reconnect
				e.Connect()
				continue
			}
			return
		}
		log.Debug("Message received, sending to receiveChannel")

		e.receiveChannel <- line
	}
}

func (e *Client) AddDevice(device *Device) {

	e.devices = append(e.devices, device)

	e.Initialize()
}

func (e *Client) Initialize() {
	if !e.dryMode {
		e.sentInitMessage()
	}
}

func (e *Client) sentInitMessage() {
	log.Debug("Sending Init Message")

	// Only init devices that are not already init
	var deviceForInit []*Device
	for i := 0; i < len(e.devices); i++ {

		// Tag required when multiple devices on same connection
		if e.devices[i].Tag == "" {
			e.devices[i].Tag = e.devices[i].UniqueID
		}

		if e.devices[i].InitDone {
			continue
		}

		e.devices[i].SetInitDone()
		deviceForInit = append(deviceForInit, e.devices[i])

	}

	if len(deviceForInit) > 1 {
		// Array of Init Messages

		var initMessages []DeviceInitMessage

		for i := 0; i < len(deviceForInit); i++ {
			initMessage := DeviceInitMessage{GenericInitMessageHeader{GenericMessageHeader{MessageType: "init"}, "json"}, *deviceForInit[i]}
			initMessages = append(initMessages, initMessage)

		}
		e.sendMessage(initMessages)

	}

	if len(deviceForInit) == 1 {
		// Only One Init Message
		initMessage := DeviceInitMessage{GenericInitMessageHeader{GenericMessageHeader{MessageType: "init"}, "json"}, *deviceForInit[0]}
		e.sendMessage(initMessage)
	} else {
		log.Warn("Cannot initialize, no devices added")
		return
	}
}

func (e *Client) processMessage(message *GenericVDCDMessage) {

	switch message.MessageType {
	case "status":
		e.processStatusMessage(message)

	case "channel":
		e.processChannelMessage(message)

	case "move":
		e.processMoveMessage(message)

	case "control":
		e.processControlMessage(message)

	case "sync":
		e.processSyncMessage(message)

	case "scenecommand":
		e.processSceneCommandMessage(message)

	case "setConfiguration":
		e.processSetConfigurationMessage(message)

	case "invokeAction":
		e.processInvokeActionMessage(message)

	case "setProperty":
		e.processSetPropertyMessage(message)

	}
}

func (e *Client) processStatusMessage(message *GenericVDCDMessage) {
	log.Debugf("Status Message. Status: %s, Error Message: %s\n", message.Status, message.ErrorMessage)
}

func (e *Client) processChannelMessage(message *GenericVDCDMessage) {
	log.Debugf("Channel Message. Index: %d, ChannelType: %d, ChannelName: %s, Value: %f, Tag: %s\n", message.Index, message.ChannelType, message.ChannelName, message.Value, message.Tag)

	// Multiple Devices available, identify by Tag
	if len(e.devices) > 1 {
		device, err := e.GetDeviceByTag(message.Tag)

		if err != nil {
			log.Warnf("Device not found by Tag %s\n", message.Tag)
			return
		}

		log.Debugf("Device found by Tag for Channel Message: %s\n", device.UniqueID)

		if device.channel_cb != nil {
			log.Debugf("Callback for Device %s set, calling it\n", device.UniqueID)
			device.channel_cb(message, device)
		}
	} else {
		// Only one device
		if e.devices[0].channel_cb != nil {
			log.Debugf("Callback for Device %s set, calling it\n", e.devices[0].UniqueID)
			e.devices[0].channel_cb(message, e.devices[0])
		}
	}

}

func (e *Client) processMoveMessage(message *GenericVDCDMessage) {
	log.Debugf("Move Message. Index: %d, Direction: %d, Tag: %s\n", message.Index, message.Direction, message.Tag)
}

func (e *Client) processControlMessage(message *GenericVDCDMessage) {
	log.Debugf("Control Message. Name: %s, Value: %f, Tag: %s\n", message.Name, message.Value, message.Tag)
}

func (e *Client) processSyncMessage(message *GenericVDCDMessage) {
	log.Debugf("Sync Message. Tag: %s\n", message.Tag)
}

func (e *Client) processSceneCommandMessage(message *GenericVDCDMessage) {
	log.Debugf("Scene Command Message. Cmd: %s Tag: %s\n", message.Cmd, message.Tag)
}

func (e *Client) processSetConfigurationMessage(message *GenericVDCDMessage) {
	log.Debugf("Scene Command Message. ConfigID: %s Tag: %s\n", message.ConfigId, message.Tag)
}

func (e *Client) processInvokeActionMessage(message *GenericVDCDMessage) {
	log.Debugf("Invoke Action Message. Params: %v Tag: %s\n", message.Params, message.Tag)
}

func (e *Client) processSetPropertyMessage(message *GenericVDCDMessage) {
	log.Debugf("Set Property Message. Property: %v Value: %f Tag: %s", message.Properties, message.Value, message.Tag)
}

func (e *Client) sendMessage(message interface{}) {

	payload, err := json.Marshal(message)

	//log.Debugf("Send Message. Raw: %s", string(payload))

	if err != nil {
		log.WithError(err).Error("Failed to Marshall object")
		return
	}

	//log.Println("Sending Message: " + string(payload))

	e.mu.Lock()
	_, err = e.w.WriteString(string(payload))

	if err == nil {
		_, err = e.w.WriteString("\r\n")
	}

	if err == nil {
		err = e.w.Flush()
	}
	e.mu.Unlock()

	if err != nil {
		log.WithError(err).Error("Send Message failed")
		return
	}

}

func (e *Client) sendByeMessage() {
	log.Infoln("Closing, sending Bye Message")

	byeMessage := GenericDeviceMessage{GenericMessageHeader: GenericMessageHeader{MessageType: "bye"}}

	e.sendMessage(byeMessage)
}

func (e *Client) sendChannelMessage(value float32, tag string, channelName string, channelType ChannelTypeType) {
	channelMessageHeader := GenericMessageHeader{MessageType: "channel"}
	channelMessageFields := GenericDeviceMessageFields{Tag: tag, ChannelName: channelName, Value: value, ChannelType: channelType}
	channelMessage := GenericDeviceMessage{channelMessageHeader, channelMessageFields}

	payload, err := json.Marshal(channelMessage)
	if err != nil {
		log.WithError(err).Error("Failed to Marshall object")
		return
	}

	log.Debugf("Send Channel Message: %s\n", string(payload))
	e.sendMessage(channelMessage)
}

func (e *Client) SendSensorMessage(value float32, tag string, channelName string, index int) {
	channelMessageHeader := GenericMessageHeader{MessageType: "sensor"}
	channelMessageFields := GenericDeviceMessageFields{Index: index, Tag: tag, ChannelName: channelName, Value: value}
	channelMessage := GenericDeviceMessage{channelMessageHeader, channelMessageFields}

	payload, err := json.Marshal(channelMessage)
	if err != nil {
		log.WithError(err).Error("Failed to Marshall object")
		return
	}

	log.Debugf("Send Sensor Message: %s\n", string(payload))
	e.sendMessage(channelMessage)
}

func (e *Client) SendButtonMessage(value float32, tag string, index int) {
	channelMessageHeader := GenericMessageHeader{MessageType: "button"}
	channelMessageFields := GenericDeviceMessageFields{Index: index, Tag: tag, Value: value}
	channelMessage := GenericDeviceMessage{channelMessageHeader, channelMessageFields}

	payload, err := json.Marshal(channelMessage)
	if err != nil {
		log.WithError(err).Error("Failed to Marshall object")
		return
	}

	log.Debugf("Send Button Message: %s\n", string(payload))
	e.sendMessage(channelMessage)

}

func (e *Client) GetDeviceByUniqueId(uniqueid string) (*Device, error) {
	for i := 0; i < len(e.devices); i++ {
		if e.devices[i].UniqueID == uniqueid {
			return e.devices[i], nil
		}
	}

	return nil, errors.New(("Device not found"))
}

func (e *Client) GetDeviceByUniqueIdAndSubDeviceIndex(uniqueid string, subDeviceIndex int) (*Device, error) {
	for i := 0; i < len(e.devices); i++ {
		if e.devices[i].UniqueID == uniqueid && e.devices[i].SubDeviceIndex == fmt.Sprintf("%d", subDeviceIndex) {
			return e.devices[i], nil
		}
	}

	return nil, errors.New(("Device not found"))
}

func (e *Client) GetDeviceByTag(tag string) (*Device, error) {
	for i := 0; i < len(e.devices); i++ {
		if e.devices[i].Tag == tag {
			return e.devices[i], nil
		}
	}

	return nil, errors.New(("Device not found"))
}

// func (e *Client) getDeviceIndex(device Device) (*int, error) {
// 	for i := 0; i < len(e.devices); i++ {
// 		if e.devices[i].UniqueID == device.UniqueID {
// 			return &i, nil
// 		}
// 	}

// 	return nil, errors.New(("Device not found"))
// }

// Send a channel message to the vdcd for the given ChannelName and ChannelType
func (e *Client) UpdateValue(device *Device, channelName string, channelType ChannelTypeType) {

	value, err := device.GetValue(channelName)

	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"UniqueID":    device.UniqueID,
			"ChannelName": channelName,
		}).Error("Value not found\n")
	}

	log.WithFields(log.Fields{
		"Name":        device.Name,
		"UniqueID":    device.UniqueID,
		"Channelname": channelName,
		"Channeltype": channelType,
		"InitDone":    device.InitDone,
	}).Infof("Update value to %f", value)

	// Make sure init is Done for the device
	if device.InitDone {
		e.sendChannelMessage(value, device.Tag, channelName, channelType)
	}

}
