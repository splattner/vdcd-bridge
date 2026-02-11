package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	mdns "github.com/hashicorp/mdns"
	log "github.com/sirupsen/logrus"
	"github.com/splattner/vdcd-bridge/pkg/vdcdapi"
)

type WledDevice struct {
	GenericDevice
	Id        string
	IPAddress string
	Name      string
	MAC       string
	Brand     string
	Product   string
}

func (w *WledDevice) NewWledDevice(vdcdClient *vdcdapi.Client, ip string, name string) *vdcdapi.Device {
	w.vdcdClient = vdcdClient
	w.IPAddress = ip
	w.Name = name
	w.Id = ip // Use IP as unique ID for now

	device := new(vdcdapi.Device)
	device.NewColorLightDevice(vdcdClient, w.Id)
	device.SetName(w.Name)
	device.SetChannelMessageCB(w.vcdcChannelCallback())
	device.ModelName = "WLED"
	device.SourceDevice = w
	device.ConfigUrl = fmt.Sprintf("http://%s", w.IPAddress)

	// Query WLED info endpoint for version and device metadata
	infoUrl := fmt.Sprintf("http://%s/json/info", w.IPAddress)
	resp, err := http.Get(infoUrl)
	if err == nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.WithError(err).Warn("WLED info response close failed")
			}
		}()
		if resp.StatusCode == 200 {
			var info struct {
				Ver     string `json:"ver"`
				Mac     string `json:"mac"`
				Brand   string `json:"brand"`
				Product string `json:"product"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&info); err == nil {
				device.ModelVersion = info.Ver
				if info.Brand != "" {
					w.Brand = info.Brand
					device.VendorName = info.Brand
				}
				if info.Product != "" {
					w.Product = info.Product
					device.ModelName = info.Product
				}
				if info.Mac != "" {
					w.MAC = info.Mac
					device.HardwareName = info.Mac
					device.UniqueID = info.Mac // Override UniqueID with MAC if available
					device.Tag = info.Mac      // Store MAC in Tag for reference
					w.Id = info.Mac            // Use MAC as unique ID if available
				}
			}
		}
	}

	w.originDevice = device
	vdcdClient.AddDevice(device)

	return device
}

func (w *WledDevice) SetValue(value float32, channelName string, channelType vdcdapi.ChannelTypeType) {
	log.Infof("Set Value for WLED Device %s to %f (channel: %s, type: %v)\n", w.Id, value, channelName, channelType)
	w.originDevice.SetValue(value, channelName)

	switch channelName {
	case "basic_switch":
		if value == 100 {
			w.TurnOn()
		} else {
			w.TurnOff()
		}
	case "brightness":
		w.SetBrightness(value)
	case "hue":
		w.SetColor(value, -1) // Only hue changed
	case "saturation":
		w.SetColor(-1, value) // Only saturation changed
	}
}

// SetBrightness sets the brightness (0-100) for the WLED device
func (w *WledDevice) SetBrightness(brightness float32) {
	url := fmt.Sprintf("http://%s/json/state", w.IPAddress)
	bri := int(brightness / 100 * 255)
	body := map[string]interface{}{"bri": bri}
	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		log.WithError(err).Error("Failed to set WLED brightness")
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("WLED brightness response close failed")
		}
	}()
	_, _ = io.ReadAll(resp.Body)
}

// SetColor sets the color using hue (0-360) and/or saturation (0-100)
func (w *WledDevice) SetColor(hue float32, saturation float32) {
	url := fmt.Sprintf("http://%s/json/state", w.IPAddress)
	// Get current color if needed (not implemented: for simplicity, just set both if provided)
	var rgb [3]int
	if hue >= 0 && saturation >= 0 {
		// Convert HSV to RGB
		r, g, b := hsvToRgb(hue, saturation, 100)
		rgb = [3]int{r, g, b}
	} else {
		// If only one is set, use defaults
		r, g, b := hsvToRgb(
			ifElse(hue >= 0, hue, 0),
			ifElse(saturation >= 0, saturation, 100),
			100,
		)
		rgb = [3]int{r, g, b}
	}
	body := map[string]interface{}{"seg": []map[string]interface{}{{"col": [][]int{rgb[:]}}}}
	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		log.WithError(err).Error("Failed to set WLED color")
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("WLED color response close failed")
		}
	}()
	_, _ = io.ReadAll(resp.Body)
}

// hsvToRgb converts HSV (hue 0-360, sat 0-100, val 0-100) to RGB (0-255)
func hsvToRgb(h, s, v float32) (int, int, int) {
	s = s / 100
	v = v / 100
	c := v * s
	x := c * (1 - float32(absInt(int((h/60))%2-1)))
	m := v - c
	var r, g, b float32
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return int((r+m)*255 + 0.5), int((g+m)*255 + 0.5), int((b+m)*255 + 0.5)
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func ifElse(cond bool, a, b float32) float32 {
	if cond {
		return a
	}
	return b
}

// DiscoverWledDevices uses mDNS to find WLED devices on the local network.
func DiscoverWledDevices(vdcdClient *vdcdapi.Client) []*vdcdapi.Device {

	log.Infoln(("Starting WLED Device discovery"))

	var devices []*vdcdapi.Device
	entriesCh := make(chan *mdns.ServiceEntry, 4)
	defer close(entriesCh)

	// WLED advertises as _wled._tcp
	go func() {
		for entry := range entriesCh {
			if !containsWledService(entry.Name) {
				continue
			}
			ip := entry.AddrV4.String()
			name := strings.ReplaceAll(entry.Name, "._wled._tcp.local.", "")
			wled := &WledDevice{}
			device := wled.NewWledDevice(vdcdClient, ip, name)
			devices = append(devices, device)
			log.Infof("Discovered WLED device: %s (%s)", name, ip)
		}
	}()

	// Start the lookup
	params := &mdns.QueryParam{
		Service:     "_wled._tcp",
		Domain:      "local",
		Timeout:     3 * 1e9, // 3 seconds
		Entries:     entriesCh,
		DisableIPv6: true,
	}
	_ = mdns.Query(params)
	return devices
}

func (w *WledDevice) TurnOn() {
	w.sendWledState(true)
}

func (w *WledDevice) TurnOff() {
	w.sendWledState(false)
}

func (w *WledDevice) sendWledState(on bool) {
	url := fmt.Sprintf("http://%s/json/state", w.IPAddress)
	body := map[string]interface{}{"on": on}
	jsonBody, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		log.WithError(err).Error("Failed to set WLED state")
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("WLED state response close failed")
		}
	}()
	_, _ = io.ReadAll(resp.Body)
}

func (w *WledDevice) StartDiscovery(vdcdClient *vdcdapi.Client) {
	DiscoverWledDevices(vdcdClient)
}

func (w *WledDevice) vcdcChannelCallback() func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {
	return func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {
		log.Debugf("WLED vcdcChannelCallback called for Device %s", device.UniqueID)
		w.SetValue(message.Value, message.ChannelName, message.ChannelType)
	}
}

// containsWledService checks if the mDNS name contains _wled._tcp
func containsWledService(name string) bool {
	return strings.Contains(name, "_wled._tcp")
}
