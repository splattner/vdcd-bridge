package discovery

type ShellyDevice struct {
	Id                   string `json:"id,omitempty"`
	Model                string `json:"model,omitempty"`
	MACAddress           string `json:"mac,omitempty"`
	IPAddress            string `json:"ip,omitempty"`
	NewFirewareAvailable bool   `json:"new_fw,omitempty"`
	FirmewareVersion     string `json:"fw_ver,omitempty"`
}

func (e *ShellyDevice) SetValue(value float32) {

}
