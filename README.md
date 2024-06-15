# External Device for plan44's vdcd to bridge between Digitalstrom and other Devices

Disclaimer: Work in Progress =) and build for my home, so only really devices I own are tested, not all use-cases covered

This Golang based project allows to integrate existing smart devices (specially lights and relays) into the [digitalSTROM](https://www.digitalstrom.com/) system. The project uses the free virtual device connector (vdc) from [plan44](https://github.com/plan44/vdcd).

Currently the following devices are supported:

* [Tasmota](https://github.com/plan44/vdcd) based relays. Uses MQTT for discovery and control of the device
* [Shelly](https://shelly.cloud/) based relays. Uses MQTT for discovery and control of the device.
* [Deconz](https://www.dresden-elektronik.de/funk/software/deconz.html) Lights and (selected) Sensors. Uses REST-AOI and Websockets
* [Zigbee2MQTT](https://www.zigbee2mqtt.io/) Lights exposing state,brightness,color_temp Featurs. Only tested with some selected models. Uses MQTT for discovery and control of the device

![DSS Devices](dss_devices.png)

## How to

### 1. start a vcdc

Clone the vdcd project:

`git clone https://github.com/plan44/vdcd.git`

Build the container image:

```bash

cd vdcd
docker build -t myImageName .
```

Start the vdcd:

```bash
 docker run --network=host -v /var/run/dbus:/var/run/dbus -v /var/run/avahi-daemon/socket:/var/run/avahi-daemon/socket myimagename vdcd --externaldevices 8999 --externalnonlocal
```

### 2. Start the vdcd Brige

Clone the vdcd-bridge Project

`git clone https://github.com/splattner/vdcd-bridge.git`

Build the binary:

```bash
cd vdcd-bridge
make $(pwd)/./vdcd-bridge-amd64
```

Start the vdcd bridge:

`./vdcd-bridge-amd64 -H ipofvdcdhost --mqtthost ip:portmqttbroker`

or as a container:

Build the container:

`make docker-build`

Start the vcdc-brige as a container:

`docker run ghcr.io/splattner/vdcd-bridge:latest-amd64 vdcd-bridge -H ipofvdcdhost --mqtthost ip:portmqttbroker`
