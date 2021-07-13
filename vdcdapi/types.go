package vdcdapi

type InitvdcMessage struct {
    Message			string	`json:"message"`
    ModelName		string	`json:"modelname,omitempty"`
	ModelVersion	string	`json:"modelVersion,omitempty"`
	IconName		string	`json:"iconname,omitempty"`
	ConfigUrl		string	`json:"configurl,omitempty"`
	AlwaysVisible	bool	`json:"alwaysVisible,omitempty"`
	Name 			string	`json:"name,omitempty"` 
}

type InitMessage struct {
    Message						string 				`json:"message"`
    Protocol					string				`json:"protocol,omitempty"`
	Tag							string				`json:"tag,omitempty"`
	UniqueID					string				`json:"uniqueid,omitempty"`
	SubDeviceIndex				int					`json:"subdeviceindex,omitempty"`
	ColorClass					int					`json:"colorclass,omitempty"`
	Group 						int					`json:"group,omitempty"`
	Output 						string				`json:"output,omitempty"`
	Kind						string				`json:"kind,omitempty"`
	EndContacts					bool				`json:"endcontacts,omitempty"`
	Move						bool				`json:"endcontacts,omitempty"`
	Sync						bool				`json:"sync,omitempty"`
	ControlValues				bool				`json:"controlvalues,omitempty"`
	SceneCommands				bool				`json:"scenecommands,omitempty"`
	Move						bool				`json:"endcontacts,omitempty"`
	Groups						int[]				`json:"groups,omitempty"`
	HardwareName				string				`json:"groups,omitempty"`
	ModelName					string				`json:"modelname,omitempty"`
	ModelVersion				string				`json:"modelversion,omitempty"`
	VendorName					string				`json:"vendorname,omitempty"`
	OemModelGUID				string				`json:"oemmodelguid,omitempty"`
	IconName					string				`json:"iconname,omitempty"`
	ConfigUrl					string				`json:"configurl,omitempty"`
	TypeIdentifier				string				`json:"typeidentifier,omitempty"`
	DeviceClass					string				`json:"deviceclass,omitempty"`
	DeviceClassVersion			int					`json:"deviceclassversion,omitempty"`
	Name 						string				`json:"name,omitempty"`
	Buttons						Button[]			`json:"buttons,omitempty"`
	Inputs						Input[]				`json:"inputs,omitempty"`
	Sensors						Sensor[]			`json:"sensors,omitempty"`
	Configurations				Configuration{}		`json:"configurations,omitempty"`
	CurrentConfigId				string				`json:"currentconfigid,omitempty"`
	Actions						Action{}			`json:"actions,omitempty"`
	DynamicActions				DyamicActions{}		`json:"dynamicactions,omitempty"`
	StandardActions				StandardActions{}	`json:"standarcactions,omitempty"`
	AutoAddStandardActions		bool 				`json:"autoaddstandardactions,omitempty"`
	NoConfirmaction				bool				`json:"noconfirmaction,omitempty"`
	States						State{} 			`json:"states,omitempty"`
	Events						Event{} 			`json:"events,omitempty"`
	Properties					Property{} 			`json:"properties,omitempty"`


}


type Button struct {
	Id				string			`json:"id,omitempty"`
	ButtonId		int				`json:"buttonid,omitempty"`
	ButtonType		int				`json:"buttontype,omitempty"`
	Element			int				`json:"element,omitempty"`
	Group			int				`json:"group,omitempty"`
	Combinables		int				`json:"combinables,omitempty"`
	LocalButton		bool			`json:"localbutton,omitempty"`
	HardwareName	string			`json:"hardwarename,omitempty"`
}

type Input struct {
	Id					string		`json:"id,omitempty"`
	InputType			int			`json:"inputtype,omitempty"`
	Usage				int			`json:"usage,omitempty"`
	Group				int			`json:"group,omitempty"`
	UpdateInterval		float32		`json:"updateinterval,omitempty"`
	AliveSignalInterval	float32		`json:"alivesignalinterval,omitempty"`
	HardwareName		string		`json:"hardwarename,omitempty"`
}

type Sensor struct {
	Id					string		`json:"id,omitempty"`
	SensorType			int			`json:"sensortype,omitempty"`
	Usage				int			`json:"usage,omitempty"`
	Group				int			`json:"group,omitempty"`
	UpdateInterval		float32		`json:"updateinterval,omitempty"`
	AliveSignalInterval	float32		`json:"alivesignalinterval,omitempty"`
	ChangesOnlyInterval	float32		`json:"changesonlyinterval,omitempty"`
	HardwareName		string		`json:"groups,omitempty"`
	Min					float32		`json:"min,omitempty"`
	Max					float32		`json:"max,omitempty"`
	Resolution			float32		`json:"resolution,omitempty"`
	
}

type Action struct {
	Description		string		`json:"description,omitempty"`
	Params			Param		`json:"params,omitempty"`
}

type StandardAction struct {
	Action		string		`json:"action"`
	title		string		`json:"title,omitempty"`
	Params		Param		`json:"params,omitempty"`	
}

type DynamicAction struct {
	Action		string		`json:"action"`
	title		string		`json:"title,omitempty"`
	Params		Param		`json:"params,omitempty"`	
}

type Configuration struct {
	Id			string		`json:"id"`
	description	string		`json:"description,omitempty"`
}

type State struct {
	description	string		`json:"description,omitempty"`
	// Todo type field
}

type Event struct {
	description	string		`json:"description,omitempty"`
	// Todo type field
}

type Property struct {
	readonly	bool		`json:"readonly,omitempty"`
	// Todo type field
}

type Param struct {
	Type			string		`json:"type"`
	SiUnit			string		`json:"siunit,omitempty"`
	Default			float32		`json:"default,omitempty"`
	Min				float32		`json:"min,omitempty"`
	Max				float32		`json:"min,omitempty"`
	Resolution		float32		`json:"resolution,omitempty"`
	Values			string[]	`json:"values,omitempty"`
}