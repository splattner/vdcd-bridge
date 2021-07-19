package vdcdapi

type ButtonType int
type ElementType int
type GroupType int
type ColorClassType int
type OutputType string
type InputType int
type UsageType int
type SensorType int
type ChanelTypeType int

const (
	NotDefinedButton ButtonType = iota
	SingleButton
	TwoWayButton
	FourWayButton
	FourWayCenterButton
	EightWayCenterButton
	OnOffButton
)

const (
	CenterElement ElementType = iota
	DownElement
	UpElement
	LeftElement
	RightElement
	UpperLeftElement
	LowerLeftElement
	UpperRightElement
	LowerRightElement
)

const (
	YellowLightGroup GroupType = iota + 1
	GreyShadowGroup
	BlueHeatingGroup
	CyanAudioGroup
	MagentaVideoGroup
	RedSecurityGroup
	GreenAccessGroup
	BlackVariableGroup
	BlueCoolingGroup
	BlueVentilationGroup
	BlueWindowsGroup
	BlueAirGroup
	RoomTemperatureGroup = 48
	RoomVentilationGroup = 49
)

const (
	YellowColorClassT ColorClassType = iota + 1
	GreyColorClassT
	BlueColorClassT
	CyanColorClassT
	MagentaColorClassT
	RedColorClassT
	GreenColorClassT
	BlackColorClassT
	WhiteColorClassT
)

const (
	LightOutput        OutputType = "light"
	ColorLightOutput              = "colorlight"
	CtLightOutput                 = "ctlight"
	MovingLightOutput             = "movinglight"
	HeatingValveOutput            = "heatingvalve"
	VentilationOutput             = "ventilation"
	FanCoilUnitOutput             = "fancoilunit"
	ShadowOutput                  = "shadow"
	ActionOutput                  = "action"
	BasicOutput                   = "basic"
)

const (
	NoSystemFunctionInput InputType = iota
	PresenceInput
	LightInput
	PresenceDarknessInput
	TwilightInput
	MotionInput
	MotionDarknessInput
	SmokeInput
	WindInput
	RainInput
	SolarRadiationInput
	ThermostatInput
	DeviceLowBatteryInput
	WindowClosedInput
	DoorClosedInput
	WindowHandleInput
	GarageDoorClosedInput
	ProtectSunlightInput
	HeatingSystemActivatedInput
	HeatingSystemChangeOverInput
	NotAllFunctionReadyInput
	MalfunctionInput
	NeedServiceInput
)

const (
	UndefinedUsage UsageType = iota
	RoomUsage
	OutdoorUsage
	UserInteractionUsage
)

const (
	UndefinedSensor SensorType = iota
	TemperatureSensor
	HumiditySensor
	IlluminationSensor
	VoltageSensor
	CarbonMonocideSensor
	RadonSensor
	GasSensor
	DustParticle10Sensor
	DustParticle2_5Sensor
	DustParticle1Sensor
	RoomOperationSensor
	FanSpeedSensor
	WindSpeedSensor
	PowerSensor
	ElectricCurrentSensor
	EnergySensor
	ElectricConsumptionSensor
	AirPressureSensor
	WindDirectionSensor
	SoundPresureLevelSensor
	PrecipitationSensor
	CarbonDioxidSensor
	GustSpeedSensor
	GustDirectionSensor
	GeneratedPowerSensor
	GeneratedEnergySensor
	WaterQuantitySensor
	WaterFlowRateSensor
	LenghtSensor
	MassSensor
	TimeSensor
)

const (
	UndefinedType ChanelTypeType = iota
	BrightnessType
	HueType
	SaturationType
	ColorTemperatureType
	XCIEColorType
	YCIEColorType
	BlindsShadePositionType
	CurtainShadePositionType
	BlindShadeAngleType
	CurtainsShadeAngleType
	AirflowIntesityType
	AirflowDirectionType
	AirflowFlapPositionType
	VentilationLouverPositionType
)

type InitvdcMessage struct {
	GenericMessageHeader
	ModelName     string `json:"modelname,omitempty"`
	ModelVersion  string `json:"modelVersion,omitempty"`
	IconName      string `json:"iconname,omitempty"`
	ConfigUrl     string `json:"configurl,omitempty"`
	AlwaysVisible bool   `json:"alwaysVisible,omitempty"`
	Name          string `json:"name,omitempty"`
}

type GenericVDCDMessage struct {
	GenericMessageHeader
	GenericVCDCMessageFields
	Status       string              `json:"status"`
	ErrorCode    int                 `json:"errorcode,omitempty"`
	ErrorDomain  string              `json:"errordomain,omitempty"`
	ErrorMessage string              `json:"errormessage,omitempty"`
	Dimming      bool                `json:"dimming,omitempty"`
	Direction    int                 `json:"direction,omitempty"`
	Name         string              `json:"name,omitempty"`
	Sync         bool                `json:"sync,omitempty"`
	Cmd          string              `json:"cmd,omitempty"`
	ConfigId     string              `json:"configid,omitempty"`
	Params       map[string]Param    `json:"params,omitempty"`
	Properties   map[string]Property `json:"properties,omitempty"`
}

type GenericVCDCMessageFields struct {
	Protocol    string         `json:"protocol,omitempty"`
	Tag         string         `json:"tag,omitempty"`
	Text        string         `json:"text,omitempty"`
	Index       int            `json:"index,omitempty"`
	ChannelName string         `json:"id,omitempty"`
	Value       float32        `json:"value,omitempty"`
	ChannelType ChanelTypeType `json:"type,omitempty"`
}

type GenericMessageHeader struct {
	MessageType string `json:"message"`
}

type GenericInitMessageHeader struct {
	GenericMessageHeader
	Protocol string `json:"protocol,omitempty"`
}

type GenericDeviceMessageFields struct {
	Tag         string         `json:"tag,omitempty"`
	Text        string         `json:"text,omitempty"`
	Index       int            `json:"index,omitempty"`
	ChannelName string         `json:"id,omitempty"`
	Value       float32        `json:"value,omitempty"`
	ChannelType ChanelTypeType `json:"type,omitempty"`
}

type GenericDeviceMessage struct {
	GenericMessageHeader
	GenericDeviceMessageFields
}

type DeviceInitMessage struct {
	GenericInitMessageHeader
	Device
}

type Device struct {
	Tag                    string                    `json:"tag,omitempty"`
	UniqueID               string                    `json:"uniqueid,omitempty"`
	SubDeviceIndex         int                       `json:"subdeviceindex,omitempty"`
	ColorClass             ColorClassType            `json:"colorclass,omitempty"`
	Group                  GroupType                 `json:"group,omitempty"`
	Output                 OutputType                `json:"output,omitempty"`
	Kind                   string                    `json:"kind,omitempty"`
	EndContacts            bool                      `json:"endcontacts,omitempty"`
	Move                   bool                      `json:"move,omitempty"`
	Sync                   bool                      `json:"sync,omitempty"`
	ControlValues          bool                      `json:"controlvalues,omitempty"`
	SceneCommands          bool                      `json:"scenecommands,omitempty"`
	Groups                 []int                     `json:"groups,omitempty"`
	HardwareName           string                    `json:"hardwarename,omitempty"`
	ModelName              string                    `json:"modelname,omitempty"`
	ModelVersion           string                    `json:"modelversion,omitempty"`
	VendorName             string                    `json:"vendorname,omitempty"`
	OemModelGUID           string                    `json:"oemmodelguid,omitempty"`
	IconName               string                    `json:"iconname,omitempty"`
	ConfigUrl              string                    `json:"configurl,omitempty"`
	TypeIdentifier         string                    `json:"typeidentifier,omitempty"`
	DeviceClass            string                    `json:"deviceclass,omitempty"`
	DeviceClassVersion     int                       `json:"deviceclassversion,omitempty"`
	Name                   string                    `json:"name,omitempty"`
	Buttons                []Button                  `json:"buttons,omitempty"`
	Inputs                 []Input                   `json:"inputs,omitempty"`
	Sensors                []Sensor                  `json:"sensors,omitempty"`
	Configurations         map[string]Configuration  `json:"configurations,omitempty"`
	CurrentConfigId        string                    `json:"currentconfigid,omitempty"`
	Actions                map[string]Action         `json:"actions,omitempty"`
	DynamicActions         map[string]DynamicAction  `json:"dynamicactions,omitempty"`
	StandardActions        map[string]StandardAction `json:"standarcactions,omitempty"`
	AutoAddStandardActions bool                      `json:"autoaddstandardactions,omitempty"`
	NoConfirmaction        bool                      `json:"noconfirmaction,omitempty"`
	States                 map[string]State          `json:"states,omitempty"`
	Events                 map[string]Event          `json:"events,omitempty"`
	Properties             map[string]Property       `json:"properties,omitempty"`

	value        float32
	client       Client
	channel_cb   func(message *GenericVDCDMessage, device *Device)
	InitDone     bool
	SourceDevice interface{}
}

type Button struct {
	Id           string      `json:"id,omitempty"`
	ButtonId     int         `json:"buttonid,omitempty"`
	ButtonType   ButtonType  `json:"buttontype,omitempty"`
	Element      ElementType `json:"element,omitempty"`
	Group        GroupType   `json:"group,omitempty"`
	Combinables  int         `json:"combinables,omitempty"`
	LocalButton  bool        `json:"localbutton,omitempty"`
	HardwareName string      `json:"hardwarename,omitempty"`
}

type Input struct {
	Id                  string  `json:"id,omitempty"`
	InputType           int     `json:"inputtype,omitempty"`
	Usage               int     `json:"usage,omitempty"`
	Group               int     `json:"group,omitempty"`
	UpdateInterval      float32 `json:"updateinterval,omitempty"`
	AliveSignalInterval float32 `json:"alivesignalinterval,omitempty"`
	HardwareName        string  `json:"hardwarename,omitempty"`
}

type Sensor struct {
	Id                  string  `json:"id,omitempty"`
	SensorType          int     `json:"sensortype,omitempty"`
	Usage               int     `json:"usage,omitempty"`
	Group               int     `json:"group,omitempty"`
	UpdateInterval      float32 `json:"updateinterval,omitempty"`
	AliveSignalInterval float32 `json:"alivesignalinterval,omitempty"`
	ChangesOnlyInterval float32 `json:"changesonlyinterval,omitempty"`
	HardwareName        string  `json:"groups,omitempty"`
	Min                 float32 `json:"min,omitempty"`
	Max                 float32 `json:"max,omitempty"`
	Resolution          float32 `json:"resolution,omitempty"`
}

type Action struct {
	Description string           `json:"description,omitempty"`
	Params      map[string]Param `json:"params,omitempty"`
}

type StandardAction struct {
	Action string           `json:"action"`
	Title  string           `json:"title,omitempty"`
	Params map[string]Param `json:"params,omitempty"`
}

type DynamicAction struct {
	Action string           `json:"action"`
	Title  string           `json:"title,omitempty"`
	Params map[string]Param `json:"params,omitempty"`
}

type Configuration struct {
	Id          string `json:"id"`
	Description string `json:"description,omitempty"`
}

type State struct {
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	SiUnit      string   `json:"siunit,omitempty"`
	Default     float32  `json:"default,omitempty"`
	Min         float32  `json:"min,omitempty"`
	Max         float32  `json:"max,omitempty"`
	Resolution  float32  `json:"resolution,omitempty"`
	Values      []string `json:"values,omitempty"`
}

type Event struct {
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	SiUnit      string   `json:"siunit,omitempty"`
	Default     float32  `json:"default,omitempty"`
	Min         float32  `json:"min,omitempty"`
	Max         float32  `json:"max,omitempty"`
	Resolution  float32  `json:"resolution,omitempty"`
	Values      []string `json:"values,omitempty"`
}

type Property struct {
	Readonly   bool     `json:"readonly,omitempty"`
	Type       string   `json:"type"`
	SiUnit     string   `json:"siunit,omitempty"`
	Default    float32  `json:"default,omitempty"`
	Min        float32  `json:"min,omitempty"`
	Max        float32  `json:"max,omitempty"`
	Resolution float32  `json:"resolution,omitempty"`
	Values     []string `json:"values,omitempty"`
}

type Param struct {
	Type       string   `json:"type"`
	SiUnit     string   `json:"siunit,omitempty"`
	Default    float32  `json:"default,omitempty"`
	Min        float32  `json:"min,omitempty"`
	Max        float32  `json:"max,omitempty"`
	Resolution float32  `json:"resolution,omitempty"`
	Values     []string `json:"values,omitempty"`
}
