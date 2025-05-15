package usb

//go:generate go run -tags generate gen.go

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/apex/log"
	"github.com/apex/log/handlers/cli"
	"github.com/apex/log/handlers/text"
	"github.com/pzl/usb/gusb"
)

func init() {
	log.SetHandler(cli.Default) // nice, but also not nice
	log.SetHandler(text.Default)
	log.SetLevel(log.InfoLevel)
	gusb.SetLogger(log.Log.(*log.Logger))
}

// Context manages all resources related to USB device handling.
type Context struct {
	done      chan struct{}
	closeOnce sync.Once

	mu      sync.Mutex
	devices map[*Device]bool
}

// NewContext returns a new Context instance.
func NewContext() *Context {
	ctx := &Context{
		done:    make(chan struct{}),
		devices: make(map[*Device]bool),
	}
	return ctx
}

// OpenDevices calls opener with each enumerated device.
// If the opener returns true, the device is opened and a Device is returned if the operation succeeds.
// Every Device returned (whether an error is also returned or not) must be closed.
// If there are any errors enumerating the devices,
// the final one is returned along with any successfully opened devices.
func (c *Context) OpenDevices(opener func(desc *Device) bool) ([]*Device, error) {
	list, err := List()
	if err != nil {
		return nil, err
	}

	var reterr error
	var ret []*Device
	for _, dev := range list {

		if !opener(dev) { // dev here is *usb.Device from List()
			continue
		}
		dev.ctx = c // Associate context with the device
		ret = append(ret, dev)
		c.mu.Lock()
		c.devices[dev] = true
		c.mu.Unlock()

	}
	return ret, reterr
}

// OpenDeviceWithVIDPID opens Device from specific VendorId and ProductId.
// If none is found, it returns nil and nil error. If there are multiple devices
// with the same VID/PID, it will return one of them, picked arbitrarily.
// If there were any errors during device list traversal, it is possible
// it will return a non-nil device and non-nil error. A Device.Close() must
// be called to release the device if the returned device wasn't nil.
func (c *Context) OpenDeviceWithVIDPID(vid, pid ID) (*Device, error) {
	var found bool
	devs, err := c.OpenDevices(func(desc *Device) bool {
		if found {
			return false
		}
		if desc.Vendor == ID(vid) && desc.Product == ID(pid) {
			found = true
			return true
		}
		return false
	})
	if len(devs) == 0 {
		return nil, err
	}
	return devs[0], nil
}

func (c *Context) closeDev(d *Device) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.devices, d)
}

func (c *Context) checkOpenDevs() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if l := len(c.devices); l > 0 {
		return fmt.Errorf("Context.Close called while %d Devices are still open, Close may be called only after all previously opened devices were successfuly closed", l)
	}
	return nil
}

// Close releases the Context and all associated resources.
func (c *Context) Close() error {
	if err := c.checkOpenDevs(); err != nil {
		return err
	}
	c.closeOnce.Do(func() {
		close(c.done)
	})
	return nil
}

// Deadline returns no deadline, as usb.Context does not support deadlines.
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

// Done returns a channel that's closed when the Context is closed.
func (c *Context) Done() <-chan struct{} {
	return c.done
}

// Err returns context.Canceled if the Context has been closed, nil otherwise.
func (c *Context) Err() error {
	select {
	case <-c.done:
		return context.Canceled
	default:
		return nil
	}
}

// Value returns nil, as usb.Context does not carry request-scoped values.
func (c *Context) Value(key any) any {
	return nil
}