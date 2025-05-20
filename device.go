package usb

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/pzl/usb/gusb"
)

// @todo: Class,Subclass,Protocol

const badIndexNumber = "invalid %s value: %d"

var (
	ErrDeviceNotFound        = errors.New("Device not found")
	ErrNoActiveConfig        = errors.New("usb: device has no active configuration")
	ErrNoInterfacesInConfig  = errors.New("usb: active configuration has no interfaces")
	ErrInvalidInterfaceIndex = errors.New("usb: interface index out of bounds")
)

type ID uint16

func (d Device) VendorName() string {
	if d.vendorNameFromIdFile != "" {
		return d.vendorNameFromIdFile
	} else {
		return d.vendorNameFromDevice
	}
}

func (d Device) ProductName() string {
	if d.productNameFromIdFile != "" {
		return d.productNameFromIdFile
	} else {
		return d.productNameFromDevice
	}
}

type Device struct {
	Bus                   int
	Device                int
	Port                  int // @todo: keep this up to date with hotplugs, resets?
	Ports                 []int
	Vendor                ID
	vendorNameFromIdFile  string
	vendorNameFromDevice  string
	Product               ID
	productNameFromIdFile string
	productNameFromDevice string
	Parent                *Device
	Speed                 Speed
	Configs               []Configuration
	ActiveConfig          *Configuration // can read SYSFSPATH/bConfigurationValue

	dataSource dataBacking
	ctx        *Context // Context that this device was opened with
	f          *os.File // USBFS file
	SysPath    string   // SYSFS directory for this device
}

func List() ([]*Device, error) {
	dd, err := gusb.Walk(nil)
	if err != nil {
		return nil, err
	}

	devs := make([]*Device, len(dd))

	for i := range dd {
		devs[i] = toDevice(dd[i])
	}
	return devs, nil
}

func Open(bus int, dev int) (*Device, error) {
	f, err := os.OpenFile(fmt.Sprintf("/dev/bus/usb/%03d/%03d", bus, dev), os.O_RDWR, 0644)
	if os.IsNotExist(err) {
		return nil, ErrDeviceNotFound
	} else if err != nil {
		log.Printf("ERROR: bus %d, dev %d: failed opening file: %v\n", bus, dev, err)
		return nil, err
	}
	desc, err := gusb.ParseDescriptor(f)
	if err != nil {
		log.Printf("ERROR: bus %d, dev %d: failed parsing descriptor: %v\n", bus, dev, err)
		return nil, err
	}
	desc.PathInfo.Bus = bus
	desc.PathInfo.Dev = dev
	d := toDevice(desc)
	d.f = f

	return d, nil
}

func VidPid(vid uint16, pid uint16) (*Device, error) {
	var dev *Device

	gusb.Walk(func(dd *gusb.DeviceDescriptor) error {
		if vid == uint16(dd.Vendor) && pid == uint16(dd.Product) {
			dev = toDevice(*dd)
			return filepath.SkipDir
		}
		return nil
	})
	if dev == nil {
		return nil, ErrDeviceNotFound
	}
	return dev, nil
}

func (d *Device) Open() error {
	if d.f != nil {
		d.f.Close()
	}

	f, err := os.OpenFile(fmt.Sprintf("/dev/bus/usb/%03d/%03d", d.Bus, d.Device), os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	d.f = f
	return nil
}

func (d *Device) Close() error {
	if d.f == nil {
		// Already closed or was never opened via d.Open()
		// If it was managed by a context, still try to deregister.
		if d.ctx != nil {
			d.ctx.closeDev(d)
			d.ctx = nil // Avoid future calls if Close is called multiple times
		}
		return nil
	}

	// Deregister from context if associated
	if d.ctx != nil {
		d.ctx.closeDev(d)
		d.ctx = nil
	}

	// @todo release any claimed interfaces. This is typically handled by the user.
	err := d.f.Close()
	d.f = nil // Mark as closed
	return err
}

func (d *Device) Interface(i int) (*Interface, error) {
	if d.ActiveConfig == nil {
		log.Printf("ERROR: interface %d: %v\n", i, ErrNoActiveConfig)
		return nil, ErrNoActiveConfig
	}
	if len(d.ActiveConfig.Interfaces) == 0 {
		// This configuration has no interfaces at all.
		return nil, ErrNoInterfacesInConfig
	}
	if i < 0 || i >= len(d.ActiveConfig.Interfaces) {
		// len > 0, but i is still out of bounds.
		return nil, fmt.Errorf("%w: index %d, available 0 to %d", ErrInvalidInterfaceIndex, i, len(d.ActiveConfig.Interfaces)-1)
	}
	return &d.ActiveConfig.Interfaces[i], nil
}

func (d *Device) DefaultInterface() (intf *Interface, done func(), err error) {
	intf, err = d.Interface(0)
	if err != nil {
		return nil, nil, err
	}
	d.Open()
	err = intf.Claim()
	if err != nil {
		return nil, nil, err
	}
	return intf, func() {
		d.Close()
		intf.Release()
	}, nil
}

// Return endpoint by its Address number.
func (d *Device) Endpoint(num int) (*Endpoint, error) {
	if num < 0 {
		return nil, fmt.Errorf(badIndexNumber, "endpoint", num)
	}
	return nil, nil // @todo, look up endpoint
}

func (d *Device) SetConfiguration(cfg int) error {
	err := d.dataSource.setConfiguration(*d, cfg)
	if err != nil {
		d.ActiveConfig = &d.Configs[cfg-1]
	}
	return err
}
func (d *Device) ClaimInterface(intf int) error { // accept int? or Interface?
	i, err := d.Interface(intf)
	if err != nil {
		return err
	}
	return i.Claim()
}
func (d *Device) ReleaseInterface(intf int) error {
	i, err := d.Interface(intf)
	if err != nil {
		return err
	}
	return i.Release()
}
func (d *Device) Reset() error {
	// https://github.com/libusb/libusb/blob/master/libusb/os/linux_usbfs.c#L1629
	return nil
}
func (d *Device) GetDriver(intf int) (string, error) {
	i, err := d.Interface(intf)
	if err != nil {
		return "", err
	}
	return i.GetDriver()
}

type Configuration struct {
	SelfPowered    bool
	RemoteWakeup   bool
	BatteryPowered bool
	MaxPower       int // in mA
	Value          int
	Interfaces     []Interface

	d *Device
}

type Speed int

const (
	SpeedUnknown Speed = iota
	SpeedLow
	SpeedFull
	SpeedHigh
	SpeedWireless
	SpeedSuper
	SpeedSuperPlus
)

func (s Speed) String() string {
	switch s {
	case SpeedUnknown:
		return "Unknown"
	case SpeedLow:
		return "Low, 1.5 Mbps"
	case SpeedFull:
		return "Full, 12Mbps"
	case SpeedHigh:
		return "High, 480 Mbps"
	case SpeedSuper:
		return "Super, 5 Gbps"
	case SpeedSuperPlus:
		return "Super Plus, 10 Gbps"
	}
	return "invalid"
}
