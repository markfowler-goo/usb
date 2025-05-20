package usb

import (
	"fmt"
)

type Interface struct {
	ID        int // interface number
	Alternate int
	Endpoints []Endpoint

	d *Device
	//@todo: isKernelDriverActive -- should it be a `Driver string` property? method? bool?
}

// Kernel interface release handled automatically
func (i *Interface) Claim() error { return backingUsbfs{}.claim(*i) }

// Kernel interface re-claim handled automatically
func (i *Interface) Release() error { return backingUsbfs{}.release(*i) }

func (i *Interface) SetAlt() error {
	return nil //@todo
}

func (i *Interface) GetDriver() (string, error) {
	return i.d.dataSource.getDriver(*i.d, i.ID)
}

func (i *Interface) GetOutEndpoint() (*OutEndpoint, error) {
	for _, ep := range i.Endpoints {
		// Check if it's an OUT endpoint (bit 7 of address is 0)
		if (ep.Address & 0x80) == 0 {
			return &OutEndpoint{Endpoint: ep}, nil
		}
	}
	return nil, fmt.Errorf("usb: no OUT endpoint found in interface %d", i.ID)
}

func (i *Interface) GetInEndpoint() (*InEndpoint, error) {
	for _, ep := range i.Endpoints {
		// Check if it's an IN endpoint (bit 7 of address is 1)
		if (ep.Address & 0x80) != 0 {
			return &InEndpoint{Endpoint: ep}, nil
		}
	}
	return nil, fmt.Errorf("usb: no IN endpoint found in interface %d", i.ID)
}
