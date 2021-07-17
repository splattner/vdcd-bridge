package discovery

import (
	"fmt"
	"log"
	"net/http"
)

type TasmotaDevice struct {
	IPAddress       string         `json:"ip,omitempty"`
	DeviceName      string         `json:"dn,omitempty"`
	FriendlyName    []string       `json:"fn,omitempty"`
	Hostname        string         `json:"hn,omitempty"`
	MACAddress      string         `json:"mac,omitempty"`
	Module          string         `json:"md,omitempty"`
	TuyaMCUFlag     int            `json:"ty,omitempty"`
	IFAN            int            `json:"if,omitempty"`
	DOffline        string         `json:"ofln,omitempty"`
	DOnline         string         `json:"onln,omitempty"`
	State           []string       `json:"st,omitempty"`
	SoftwareVersion string         `json:"sw,omitempty"`
	Topic           string         `json:"t,omitempty"`
	Fulltopic       string         `json:"ft,omitempty"`
	TopicPrefix     []string       `json:"tp,omitempty"`
	Relays          []int          `json:"rl,omitempty"`
	Switches        []int          `json:"swc,omitempty"`
	SWN             []int          `json:"swn,omitempty"`
	Buttons         []int          `json:"btn,omitempty"`
	SetOptions      map[string]int `json:"so,omitempty"`
	LK              int            `json:"lk,omitempty"`
	LightSubtype    int            `json:"lt_st,omitempty"`
	ShutterOptions  []int          `json:"sho,omitempty"`
	Version         int            `json:"ver,omitempty"`
}

func (e *TasmotaDevice) SetValue(value float32) {

	var urlOn = "http://%s/cm?cmnd=Power%%20On"
	var urlOff = "http://%s/cm?cmnd=Power%%20Off"

	if value == 100 {
		log.Printf("Calling On for Device %s\n", e.FriendlyName)
		http.Get(fmt.Sprintf(urlOn, e.IPAddress))
	} else {
		log.Printf("Calling Off for Device %s\n", e.FriendlyName)
		http.Get(fmt.Sprintf(urlOff, e.IPAddress))
	}

}
