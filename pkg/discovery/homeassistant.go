package discovery

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/splattner/vdcd-bridge/pkg/vdcdapi"
)

const homeAssistantLabel = "digitalstrom"

type HomeAssistantDevice struct {
	GenericDevice
	baseURL  string
	token    string
	entityID string
	name     string

	minMireds int
	maxMireds int

	devices         map[string]*HomeAssistantDevice
	devicesMu       sync.RWMutex
	listenerStarted bool

	supportsBrightness bool
	supportsColorTemp  bool
	supportsColor      bool
}

type haEntityRegistryEntry struct {
	EntityID     string   `json:"entity_id"`
	Name         string   `json:"name"`
	OriginalName string   `json:"original_name"`
	DeviceID     string   `json:"device_id"`
	Labels       []string `json:"labels"`
}

type haState struct {
	EntityID   string          `json:"entity_id"`
	State      string          `json:"state"`
	Attributes haStateAttrs    `json:"attributes"`
	Context    json.RawMessage `json:"context"`
}

type haStateAttrs struct {
	FriendlyName       string    `json:"friendly_name"`
	SupportedColorMode []string  `json:"supported_color_modes"`
	Brightness         *int      `json:"brightness"`
	ColorTemp          *int      `json:"color_temp"`
	HSColor            []float64 `json:"hs_color"`
	ColorMode          string    `json:"color_mode"`
	MinMireds          *int      `json:"min_mireds"`
	MaxMireds          *int      `json:"max_mireds"`
}

type haWSMessage struct {
	Type    string          `json:"type"`
	ID      int             `json:"id,omitempty"`
	Success *bool           `json:"success,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *haWSError      `json:"error,omitempty"`
}

type haWSError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type haWSEventMessage struct {
	Type  string  `json:"type"`
	Event haEvent `json:"event"`
}

type haEvent struct {
	EventType string      `json:"event_type"`
	Data      haEventData `json:"data"`
}

type haEventData struct {
	EntityID string  `json:"entity_id"`
	NewState haState `json:"new_state"`
}

func (e *HomeAssistantDevice) StartDiscovery(vdcdClient *vdcdapi.Client, baseURL string, token string) {
	e.vdcdClient = vdcdClient
	e.baseURL = strings.TrimRight(baseURL, "/")
	e.token = token

	if e.baseURL == "" || e.token == "" {
		log.Warn("Home Assistant discovery skipped: base URL or token missing")
		return
	}

	log.WithField("baseURL", e.baseURL).Info("Starting Home Assistant light discovery")

	if e.devices == nil {
		e.devices = make(map[string]*HomeAssistantDevice)
	}

	if err := e.discoverAndRegister(); err != nil {
		log.WithError(err).Error("Home Assistant entity registry fetch failed")
		return
	}

	if !e.listenerStarted {
		e.devicesMu.RLock()
		deviceCount := len(e.devices)
		e.devicesMu.RUnlock()
		if deviceCount > 0 {
			e.listenerStarted = true
			go e.listenStateChanges()
		}
	}
}

func (e *HomeAssistantDevice) vcdcChannelCallback() func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {
	f := func(message *vdcdapi.GenericVDCDMessage, device *vdcdapi.Device) {
		log.Debugf("Home Assistant vcdcCallBack called for Device %s\n", device.UniqueID)
		e.SetValue(message.Value, message.ChannelName, message.ChannelType)
	}

	return f
}

func (e *HomeAssistantDevice) SetValue(value float32, channelName string, channelType vdcdapi.ChannelTypeType) {
	log.Infof("Home Assistant Set Value for %s to %f on Channel '%s'", e.entityID, value, channelName)

	e.originDevice.SetValue(value, channelName)

	switch channelName {
	case "basic_switch":
		if value > 0 {
			e.TurnOn(nil)
		} else {
			e.TurnOff()
		}
	case "brightness":
		if value <= 0 {
			e.TurnOff()
			return
		}
		brightness := float32(value)
		e.TurnOn(map[string]interface{}{"brightness_pct": brightness})
	case "colortemp":
		mireds := kelvinToMireds(int(value))
		mireds = clampMireds(mireds, e.minMireds, e.maxMireds)
		e.TurnOn(map[string]interface{}{"color_temp": mireds})
	case "hue":
		sat, _ := e.originDevice.GetValue("saturation")
		e.TurnOn(map[string]interface{}{"hs_color": []float32{value, sat}})
	case "saturation":
		hue, _ := e.originDevice.GetValue("hue")
		e.TurnOn(map[string]interface{}{"hs_color": []float32{hue, value}})
	}
}

func (e *HomeAssistantDevice) TurnOn(extra map[string]interface{}) {
	payload := map[string]interface{}{"entity_id": e.entityID}
	for k, v := range extra {
		payload[k] = v
	}

	if err := e.callService("light", "turn_on", payload); err != nil {
		log.WithError(err).WithField("entity", e.entityID).Error("Home Assistant turn_on failed")
	}
}

func (e *HomeAssistantDevice) TurnOff() {
	payload := map[string]interface{}{"entity_id": e.entityID}
	if err := e.callService("light", "turn_off", payload); err != nil {
		log.WithError(err).WithField("entity", e.entityID).Error("Home Assistant turn_off failed")
	}
}

func (e *HomeAssistantDevice) applyInitialState(state haState) {
	switch state.State {
	case "on":
		e.originDevice.UpdateValue(100, "basic_switch", vdcdapi.UndefinedType)
	case "off":
		e.originDevice.UpdateValue(0, "basic_switch", vdcdapi.UndefinedType)
	}

	if state.Attributes.Brightness != nil {
		brightnessPct := float32(*state.Attributes.Brightness) / 255 * 100
		e.originDevice.UpdateValue(brightnessPct, "brightness", vdcdapi.BrightnessType)
	}

	if state.Attributes.ColorTemp != nil {
		kelvin := miredsToKelvin(*state.Attributes.ColorTemp)
		e.originDevice.UpdateValue(float32(kelvin), "colortemp", vdcdapi.ColorTemperatureType)
	}

	if len(state.Attributes.HSColor) == 2 {
		e.originDevice.UpdateValue(float32(state.Attributes.HSColor[0]), "hue", vdcdapi.HueType)
		e.originDevice.UpdateValue(float32(state.Attributes.HSColor[1]), "saturation", vdcdapi.SaturationType)
	}
}

func (e *HomeAssistantDevice) applyStateUpdate(state haState) {
	switch state.State {
	case "on":
		e.originDevice.UpdateValue(100, "basic_switch", vdcdapi.UndefinedType)
	case "off":
		e.originDevice.UpdateValue(0, "basic_switch", vdcdapi.UndefinedType)
	}

	if state.Attributes.Brightness != nil {
		brightnessPct := float32(*state.Attributes.Brightness) / 255 * 100
		e.originDevice.UpdateValue(brightnessPct, "brightness", vdcdapi.BrightnessType)
	}

	if state.Attributes.ColorTemp != nil {
		kelvin := miredsToKelvin(*state.Attributes.ColorTemp)
		e.originDevice.UpdateValue(float32(kelvin), "colortemp", vdcdapi.ColorTemperatureType)
	}

	if len(state.Attributes.HSColor) == 2 {
		e.originDevice.UpdateValue(float32(state.Attributes.HSColor[0]), "hue", vdcdapi.HueType)
		e.originDevice.UpdateValue(float32(state.Attributes.HSColor[1]), "saturation", vdcdapi.SaturationType)
	}
}

func (e *HomeAssistantDevice) discoverAndRegister() error {
	entityEntries, err := e.fetchEntityRegistry()
	if err != nil {
		return err
	}

	for _, entry := range entityEntries {
		if !strings.HasPrefix(entry.EntityID, "light.") {
			continue
		}

		if !hasLabel(entry.Labels, homeAssistantLabel) {
			continue
		}

		e.devicesMu.RLock()
		_, exists := e.devices[entry.EntityID]
		e.devicesMu.RUnlock()
		if exists {
			continue
		}

		state, err := e.fetchState(entry.EntityID)
		if err != nil {
			log.WithError(err).WithField("entity", entry.EntityID).Warn("Home Assistant state fetch failed")
			continue
		}

		haDevice := new(HomeAssistantDevice)
		haDevice.vdcdClient = e.vdcdClient
		haDevice.baseURL = e.baseURL
		haDevice.token = e.token
		haDevice.entityID = entry.EntityID
		haDevice.name = resolveHAName(entry, state)
		haDevice.supportsBrightness, haDevice.supportsColorTemp, haDevice.supportsColor = detectLightCapabilities(state.Attributes.SupportedColorMode)
		haDevice.minMireds = normalizeMireds(state.Attributes.MinMireds, 153)
		haDevice.maxMireds = normalizeMireds(state.Attributes.MaxMireds, 500)

		device := new(vdcdapi.Device)
		device.SetChannelMessageCB(haDevice.vcdcChannelCallback())
		device.SourceDevice = haDevice

		uniqueID := haDevice.entityID
		switch {
		case haDevice.supportsColor:
			device.NewColorLightDevice(e.vdcdClient, uniqueID)
		case haDevice.supportsColorTemp:
			device.NewCTLightDevice(e.vdcdClient, uniqueID)
		case haDevice.supportsBrightness:
			device.NewLightDevice(e.vdcdClient, uniqueID, true)
		default:
			device.NewLightDevice(e.vdcdClient, uniqueID, false)
		}

		device.SetName(haDevice.name)
		device.ModelName = "Home Assistant"
		device.ConfigUrl = haDevice.baseURL

		haDevice.originDevice = device

		_, notfounderr := e.vdcdClient.GetDeviceByUniqueId(uniqueID)
		if notfounderr != nil {
			log.WithFields(log.Fields{
				"entity": haDevice.entityID,
				"name":   haDevice.name,
			}).Info("Home Assistant light discovered")
			e.vdcdClient.AddDevice(device)
		}

		haDevice.applyInitialState(state)

		e.devicesMu.Lock()
		e.devices[haDevice.entityID] = haDevice
		e.devicesMu.Unlock()
	}

	return nil
}

func (e *HomeAssistantDevice) fetchEntityRegistry() ([]haEntityRegistryEntry, error) {
	resp, err := e.doRequest("GET", "/api/config/entity_registry/list", nil)
	if err == nil {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				log.WithError(err).Warn("Home Assistant entity registry response close failed")
			}
		}()

		if resp.StatusCode < 300 {
			var entries []haEntityRegistryEntry
			if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
				return nil, err
			}
			return entries, nil
		}

		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("entity registry error: %s", string(body))
		}
	}

	log.Debug("Home Assistant entity registry REST endpoint unavailable, falling back to WebSocket")
	return e.fetchEntityRegistryWS()
}

func (e *HomeAssistantDevice) fetchState(entityID string) (haState, error) {
	endpoint := fmt.Sprintf("/api/states/%s", url.PathEscape(entityID))
	resp, err := e.doRequest("GET", endpoint, nil)
	if err != nil {
		return haState{}, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Home Assistant state response close failed")
		}
	}()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return haState{}, fmt.Errorf("state error: %s", string(body))
	}

	var state haState
	if err := json.NewDecoder(resp.Body).Decode(&state); err != nil {
		return haState{}, err
	}

	return state, nil
}

func (e *HomeAssistantDevice) callService(domain string, service string, payload map[string]interface{}) error {
	endpoint := fmt.Sprintf("/api/services/%s/%s", domain, service)
	resp, err := e.doRequest("POST", endpoint, payload)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Warn("Home Assistant service response close failed")
		}
	}()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("service error: %s", string(body))
	}

	return nil
}

func (e *HomeAssistantDevice) doRequest(method string, endpoint string, body interface{}) (*http.Response, error) {
	requestURL := fmt.Sprintf("%s%s", e.baseURL, endpoint)
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewBuffer(payload)
	}

	req, err := http.NewRequest(method, requestURL, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if e.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.token))
	}

	client := &http.Client{}
	return client.Do(req)
}

func (e *HomeAssistantDevice) fetchEntityRegistryWS() ([]haEntityRegistryEntry, error) {
	wsURL, err := toWebSocketURL(e.baseURL, "/api/websocket")
	if err != nil {
		return nil, err
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.WithError(err).Warn("Home Assistant websocket close failed")
		}
	}()

	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, err
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	var hello haWSMessage
	if err := json.Unmarshal(msg, &hello); err != nil {
		return nil, err
	}
	if hello.Type != "auth_required" && hello.Type != "auth_ok" {
		return nil, fmt.Errorf("unexpected websocket hello: %s", hello.Type)
	}

	if hello.Type == "auth_required" {
		if err := conn.WriteJSON(map[string]interface{}{"type": "auth", "access_token": e.token}); err != nil {
			return nil, err
		}

		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return nil, err
		}
		_, msg, err = conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		var authResp haWSMessage
		if err := json.Unmarshal(msg, &authResp); err != nil {
			return nil, err
		}
		if authResp.Type != "auth_ok" {
			if authResp.Type == "auth_invalid" {
				return nil, fmt.Errorf("home assistant auth failed")
			}
			return nil, fmt.Errorf("unexpected auth response: %s", authResp.Type)
		}
	}

	requestID := 1
	if err := conn.WriteJSON(map[string]interface{}{"id": requestID, "type": "config/entity_registry/list"}); err != nil {
		return nil, err
	}

	for {
		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return nil, err
		}
		_, msg, err = conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		var resp haWSMessage
		if err := json.Unmarshal(msg, &resp); err != nil {
			return nil, err
		}
		if resp.Type != "result" || resp.ID != requestID {
			continue
		}
		if resp.Success != nil && !*resp.Success {
			if resp.Error != nil {
				return nil, fmt.Errorf("websocket error: %s", resp.Error.Message)
			}
			return nil, fmt.Errorf("websocket result failed")
		}
		var entries []haEntityRegistryEntry
		if err := json.Unmarshal(resp.Result, &entries); err != nil {
			return nil, err
		}
		return entries, nil
	}
}

func (e *HomeAssistantDevice) listenStateChanges() {
	wsURL, err := toWebSocketURL(e.baseURL, "/api/websocket")
	if err != nil {
		log.WithError(err).Error("Home Assistant websocket URL invalid")
		return
	}

	backoff := 2 * time.Second
	maxBackoff := 60 * time.Second

	for {
		if err := e.listenStateChangesOnce(wsURL); err != nil {
			log.WithError(err).Warn("Home Assistant websocket disconnected, retrying")
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = 2 * time.Second
	}
}

func (e *HomeAssistantDevice) listenStateChangesOnce(wsURL string) error {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.WithError(err).Warn("Home Assistant websocket close failed")
		}
	}()

	if err := e.websocketAuth(conn); err != nil {
		return err
	}

	requestID := 1
	if err := conn.WriteJSON(map[string]interface{}{"id": requestID, "type": "subscribe_events", "event_type": "state_changed"}); err != nil {
		return err
	}

	for {
		if err := conn.SetReadDeadline(time.Now().Add(90 * time.Second)); err != nil {
			return err
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var eventMsg haWSEventMessage
		if err := json.Unmarshal(msg, &eventMsg); err != nil {
			continue
		}
		if eventMsg.Type != "event" || eventMsg.Event.EventType != "state_changed" {
			continue
		}

		entityID := eventMsg.Event.Data.EntityID
		e.devicesMu.RLock()
		device, ok := e.devices[entityID]
		e.devicesMu.RUnlock()
		if !ok {
			continue
		}

		device.applyStateUpdate(eventMsg.Event.Data.NewState)
	}
}

func (e *HomeAssistantDevice) websocketAuth(conn *websocket.Conn) error {
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		return err
	}
	var hello haWSMessage
	if err := json.Unmarshal(msg, &hello); err != nil {
		return err
	}
	if hello.Type != "auth_required" && hello.Type != "auth_ok" {
		return fmt.Errorf("unexpected websocket hello: %s", hello.Type)
	}

	if hello.Type == "auth_required" {
		if err := conn.WriteJSON(map[string]interface{}{"type": "auth", "access_token": e.token}); err != nil {
			return err
		}

		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return err
		}
		_, msg, err = conn.ReadMessage()
		if err != nil {
			return err
		}
		var authResp haWSMessage
		if err := json.Unmarshal(msg, &authResp); err != nil {
			return err
		}
		if authResp.Type != "auth_ok" {
			if authResp.Type == "auth_invalid" {
				return fmt.Errorf("home assistant auth failed")
			}
			return fmt.Errorf("unexpected auth response: %s", authResp.Type)
		}
	}

	return nil
}

func toWebSocketURL(baseURL string, path string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	u.Path = path
	u.RawQuery = ""
	return u.String(), nil
}

func detectLightCapabilities(colorModes []string) (hasBrightness bool, hasColorTemp bool, hasColor bool) {
	for _, mode := range colorModes {
		switch mode {
		case "color_temp":
			hasColorTemp = true
			hasBrightness = true
		case "hs", "rgb", "xy", "rgbw", "rgbww":
			hasColor = true
			hasBrightness = true
		case "brightness":
			hasBrightness = true
		case "onoff":
			// no-op
		}
	}
	return hasBrightness, hasColorTemp, hasColor
}

func hasLabel(labels []string, label string) bool {
	for _, entry := range labels {
		if strings.EqualFold(entry, label) {
			return true
		}
	}
	return false
}

func resolveHAName(entry haEntityRegistryEntry, state haState) string {
	if state.Attributes.FriendlyName != "" {
		return state.Attributes.FriendlyName
	}
	if entry.Name != "" {
		return entry.Name
	}
	if entry.OriginalName != "" {
		return entry.OriginalName
	}
	return entry.EntityID
}

func miredsToKelvin(mireds int) int {
	if mireds <= 0 {
		return 0
	}
	return int(1000000 / mireds)
}

func kelvinToMireds(kelvin int) int {
	if kelvin <= 0 {
		return 0
	}
	return int(1000000 / kelvin)
}

func clampMireds(value int, min int, max int) int {
	if min > 0 && value < min {
		return min
	}
	if max > 0 && value > max {
		return max
	}
	return value
}

func normalizeMireds(value *int, fallback int) int {
	if value == nil || *value == 0 {
		return fallback
	}
	return *value
}
