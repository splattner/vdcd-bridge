package discovery

import (
	"fmt"
	"strings"

	deconzgroup "github.com/jurgen-kluft/go-conbee/groups"
	log "github.com/sirupsen/logrus"
	"github.com/splattner/vdcd-bridge/pkg/vdcdapi"
)

func (e *DeconzDevice) NewDeconzGroupDevice() *vdcdapi.Device {

	device := new(vdcdapi.Device)

	device.SetChannelMessageCB(e.vcdcChannelCallback())

	// Group only allows for on/off -> basic switch, no dimming
	device.NewLightDevice(e.vdcdClient, fmt.Sprintf("%d", e.group.ID), false)

	device.ModelName = "Light Group"
	device.SetName(fmt.Sprintf("Group: %s", e.group.Name))

	device.ConfigUrl = fmt.Sprintf("http://%s:%d", e.deconzHost, e.deconzPort)
	device.SourceDevice = e

	e.originDevice = device
	e.vdcdClient.AddDevice(device)

	return device
}

func (e *DeconzDevice) groupsDiscovery(group deconzgroup.Group) {

	if len(group.Lights) > 0 {

		deconzDeviceGroup := new(DeconzDevice)
		deconzDeviceGroup.IsGroup = true
		deconzDeviceGroup.group = group

		log.Infof("Deconz, Group discovered: Name: %s, \n", group.Name)

		_, notfounderr := e.vdcdClient.GetDeviceByUniqueId(fmt.Sprint(group.ID))
		if notfounderr != nil {
			log.Debugf("Deconz, Device not found in vcdc -> Adding \n")
			deconzDeviceGroup.NewDeconzDevice(e.vdcdClient, e.deconzHost, e.deconzPort, e.deconzWebSocketPort, e.deconzAPI)
		}

		e.allDeconzDevices = append(e.allDeconzDevices, *deconzDeviceGroup)

	}

}

func (e *DeconzDevice) groupStateChangedCallback(state *DeconzState) {

	log.Debugf("Deconz, groupStateChangedCallback called for Device '%s'. State: '%+v'\n", e.light.Name, state)

	if state.AllOn {
		e.originDevice.UpdateValue(100, "basic_switch", vdcdapi.UndefinedType)
	}

	if state.AnyOn {
		e.originDevice.UpdateValue(100, "basic_switch", vdcdapi.UndefinedType)
	}

	if !state.AnyOn {
		e.originDevice.UpdateValue(0, "basic_switch", vdcdapi.UndefinedType)
	}

}

func (e *DeconzDevice) setGroupState() {

	state := strings.Replace(e.group.Action.String(), "\n", ",", -1)
	state = strings.Replace(state, " ", "", -1)

	log.Infof("Deconz, call SetGroupState with state (%s) for Light with id %d\n", state, e.group.ID)

	conbeehost := fmt.Sprintf("%s:%d", e.deconzHost, e.deconzPort)
	ll := deconzgroup.New(conbeehost, e.deconzAPI)
	_, err := ll.SetGroupState(e.light.ID, e.group.Action)
	if err != nil {
		log.Debugln("Deconz, SetGroupState Error", err)
	}
}
