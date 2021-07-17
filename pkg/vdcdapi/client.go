package vdcdapi

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type Client struct {
	conn net.Conn
	host string
	port int

	r *bufio.Reader
	w *bufio.Writer
	sync.Mutex

	dialRetry int

	devices []Device

	modelName  string
	vendorName string
}

func (e *Client) NewCient(host string, port int, modelName string, vendorName string) {
	e.host = host
	e.port = port
	e.dialRetry = 5

	e.modelName = modelName
	e.vendorName = vendorName
}

func (e *Client) Connect() {

	var connString = e.host + ":" + fmt.Sprint((e.port))
	var conn net.Conn
	var err error

	log.Println("Trying to connect to vcdc: " + connString)

	for i := 0; i < e.dialRetry; i++ {

		conn, err = net.Dial("tcp", connString)

		if err != nil {
			log.Println("Dial failed:", err.Error())
			time.Sleep(time.Second)
		} else {
			break
		}

	}

	if conn == nil {
		log.Println("Failed to connect to vcdc: " + connString)
		os.Exit(1)
	}

	log.Println("Connected to vcdc: " + connString)

	e.conn = conn
	e.r = bufio.NewReader(e.conn)
	e.w = bufio.NewWriter(e.conn)
}

func (e *Client) Close() {
	e.Lock()
	defer e.Unlock()
	log.Println("Closing connection from vcdc")
	e.sendByeMessage()
	e.conn.Close()
}

func (e *Client) Listen() {

	log.Println("Start listening")

	for {
		line, err := e.r.ReadString('\n')

		if err != nil {
			log.Println("Failed to read: " + err.Error())

			if err == io.EOF {
				// try to reconnect
				e.Connect()
				continue
			}
			return
		}

		var msg GenericVDCDMessage
		err = json.Unmarshal([]byte(line), &msg)

		if err != nil {
			log.Println("Json Unmarshal failed:", err.Error())
		}

		e.processMessage(&msg)

	}

}

func (e *Client) AddDevice(device Device) {

	e.devices = append(e.devices, device)

	e.Initialize()
}

func (e *Client) Initialize() {
	e.sentInitMessage()
}

func (e *Client) sentInitMessage() {
	log.Println("Sending Init Message")

	// Only init devices that are not already init
	var deviceForInit []Device
	for i := 0; i < len(e.devices); i++ {

		// Tag required when multiple devices on same connection
		if e.devices[i].Tag == "" {
			e.devices[i].Tag = e.devices[i].UniqueID
		}

		if e.devices[i].initDone {
			continue
		}

		deviceForInit = append(deviceForInit, e.devices[i])
		e.devices[i].initDone = true
	}

	if len(deviceForInit) > 1 {
		// Array of Init Messages

		var initMessages []DeviceInitMessage

		for i := 0; i < len(deviceForInit); i++ {
			initMessage := DeviceInitMessage{GenericInitMessageHeader{GenericMessageHeader{MessageType: "init"}, "json"}, deviceForInit[i]}
			initMessages = append(initMessages, initMessage)
		}
		e.sendMessage(initMessages)

	}

	if len(deviceForInit) == 1 {
		// Only One Init Message
		initMessage := DeviceInitMessage{GenericInitMessageHeader{GenericMessageHeader{MessageType: "init"}, "json"}, deviceForInit[0]}

		e.sendMessage(initMessage)
	} else {
		log.Println("Cannot initialize, no devices added")
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
	log.Printf("Status Message. Status %s, Error Message %s", message.Status, message.ErrorMessage)
}

func (e *Client) processChannelMessage(message *GenericVDCDMessage) {
	log.Printf("Channel Message. Index: %d, ChannelType: %d, Id: %s, Value: %f, Tag: %s\n", message.Index, message.ChannelType, message.ChannelName, message.Value, message.Tag)

	// Multiple Devices available, identify by Tag
	if len(e.devices) > 1 {
		device, err := e.GetDeviceByTag(message.Tag)

		if err != nil {
			log.Printf("Device not found by Tag %s\n", message.Tag)
		}

		if device.channel_cb != nil {
			device.channel_cb(message, device)
		}
	} else {
		// Only one device
		if e.devices[0].channel_cb != nil {
			e.devices[0].channel_cb(message, &e.devices[0])
		}
	}

}

func (e *Client) processMoveMessage(message *GenericVDCDMessage) {
	log.Printf("Move Message. Index: %d, Direction: %d, Tag: %s\n", message.Index, message.Direction, message.Tag)
}

func (e *Client) processControlMessage(message *GenericVDCDMessage) {
	log.Printf("Control Message. Name: %s, Value: %f, Tag: %s\n", message.Name, message.Value, message.Tag)
}

func (e *Client) processSyncMessage(message *GenericVDCDMessage) {
	log.Printf("Sync Message. Tag: %s\n", message.Tag)
}

func (e *Client) processSceneCommandMessage(message *GenericVDCDMessage) {
	log.Printf("Scene Command Message. Cmd: %s Tag: %s\n", message.Cmd, message.Tag)
}

func (e *Client) processSetConfigurationMessage(message *GenericVDCDMessage) {
	log.Printf("Scene Command Message. ConfigID: %s Tag: %s\n", message.ConfigId, message.Tag)
}

func (e *Client) processInvokeActionMessage(message *GenericVDCDMessage) {
	log.Printf("Invoke Action Message. Params: %v Tag: %s\n", message.Params, message.Tag)
}

func (e *Client) processSetPropertyMessage(message *GenericVDCDMessage) {
	log.Printf("Set Property Message. Property: %v Value: %f Tag: %s", message.Properties, message.Value, message.Tag)
}

func (e *Client) sendMessage(message interface{}) {

	payload, err := json.Marshal(message)

	if err != nil {
		log.Println("Failed to Marshall object")
	}

	log.Println("Sending Message: " + string(payload))

	e.Lock()
	_, err = e.w.WriteString(string(payload))

	if err == nil {
		_, err = e.w.WriteString("\r\n")
	}

	if err == nil {
		err = e.w.Flush()
	}
	e.Unlock()

	if err != nil {
		log.Println("Send Message failed:", err.Error())
		os.Exit(1)
	}

}

func (e *Client) sendByeMessage() {
	log.Println("Sending Bye Message")

	byeMessage := GenericDeviceMessage{GenericMessageHeader: GenericMessageHeader{MessageType: "bye"}}

	e.sendMessage(byeMessage)
}

func (e *Client) sendChannelMessageByIndex(index int, value float32, tag string) {
	log.Println("Sending Channel Message byIndex")

	channelMessage := GenericDeviceMessage{GenericMessageHeader: GenericMessageHeader{MessageType: "channel"}, GenericDeviceMessageFields: GenericDeviceMessageFields{Index: index, Value: value, Tag: tag}}

	e.sendMessage(channelMessage)
}

func (e *Client) sendChannelMessageByChannelName(channelName string, value float32, tag string) {
	log.Println("Sending Channel Message by ChannelName")

	channelMessage := GenericDeviceMessage{GenericMessageHeader: GenericMessageHeader{MessageType: "channel"}, GenericDeviceMessageFields: GenericDeviceMessageFields{ChannelName: channelName, Value: value, Tag: tag}}
	e.sendMessage(channelMessage)

}

func (e *Client) sendChannelMessageByChannelType(channelType ChanelTypeType, value float32, tag string) {
	log.Println("Sending Channel Message by Type")

	channelMessage := GenericDeviceMessage{GenericMessageHeader: GenericMessageHeader{MessageType: "channel"}, GenericDeviceMessageFields: GenericDeviceMessageFields{ChannelType: channelType, Value: value, Tag: tag}}
	e.sendMessage(channelMessage)

}

func (e *Client) GetDeviceByUniqueId(uniqueid string) (*Device, error) {
	for i := 0; i < len(e.devices); i++ {
		if e.devices[i].UniqueID == uniqueid {
			return &e.devices[i], nil
		}
	}

	return nil, errors.New(("Device not found"))
}

func (e *Client) GetDeviceByTag(tag string) (*Device, error) {
	for i := 0; i < len(e.devices); i++ {
		if e.devices[i].Tag == tag {
			return &e.devices[i], nil
		}
	}

	return nil, errors.New(("Device not found"))
}

func (e *Client) getDeviceIndex(device Device) (*int, error) {
	for i := 0; i < len(e.devices); i++ {
		if e.devices[i].UniqueID == device.UniqueID {
			return &i, nil
		}
	}

	return nil, errors.New(("Device not found"))
}

func (e *Client) UpdateValue(device Device) {

	index, _ := e.getDeviceIndex(device)

	e.sendChannelMessageByIndex(*index, device.value, device.Tag)
}
