package usb

import (
	"context"
	"errors"
	"fmt"

	"github.com/pzl/usb/gusb"
)

type Endpoint struct {
	// Address is the endpoint address, including the direction bit (bit 7: 0 for OUT, 1 for IN).
	Address          int
	TransferType     int
	MaxPacketSize    int
	MaxISOPacketSize int

	i *Interface
}

type OutEndpoint struct {
	Endpoint
}

type InEndpoint struct {
	Endpoint
}

// TransferTypeBulk defines the bulk transfer type for an endpoint.
// (Value is 0x02 as per USB specification section 9.6.6 bmAttributes bits 1..0,
// and matches gusb.EndpointTypeBulk)
const TransferTypeBulk = 0x02

/* ---- Synchronous Sending ---- */

func (e *Endpoint) CtrlTransfer() {
	// @todo: Implement control transfer
}

// BulkOut sends data to a bulk OUT endpoint.
// It takes the data to send and a timeout in milliseconds.
// It returns the number of bytes written and an error if one occurred.
func (e *OutEndpoint) BulkOut(data []byte, timeoutMs int) (int, error) {
	if e.i == nil || e.i.d == nil || e.i.d.f == nil {
		return 0, errors.New("usb: device not open for BulkOut")
	}

	// Check if it's an OUT endpoint (bit 7 of address is 0)
	if (e.Address & 0x80) != 0 {
		return 0, fmt.Errorf("usb: endpoint address %02X is not an OUT endpoint", e.Address)
	}

	// Check if it's a Bulk endpoint
	if e.TransferType != TransferTypeBulk {
		return 0, fmt.Errorf("usb: endpoint address %02X is not a bulk endpoint (type %02X)", e.Address, e.TransferType)
	}

	bt := gusb.BulkTransfer{
		Ep:      uint32(e.Address), // Endpoint address including direction
		Len:     uint32(len(data)),
		Timeout: uint32(timeoutMs),
		Data:    gusb.SlicePtr(data),
	}

	n, err := gusb.Ioctl(e.i.d.f, gusb.USBDEVFS_BULK, &bt)
	if err != nil {
		return n, fmt.Errorf("usb: BulkOut to ep %02X failed: %w", e.Address, err)
	}
	return n, nil
}

// BulkIn receives data from a bulk IN endpoint.
// It takes a buffer to fill and a timeout in milliseconds.
// The size of the buffer determines the maximum amount of data to read.
// It returns the number of bytes read into the buffer and an error if one occurred.
func (e *InEndpoint) BulkIn(buffer []byte, timeoutMs int) (int, error) {
	if e.i == nil || e.i.d == nil || e.i.d.f == nil {
		return 0, errors.New("usb: device not open for BulkIn")
	}

	// Check if it's an IN endpoint (bit 7 of address is 1)
	if (e.Address & 0x80) == 0 {
		return 0, fmt.Errorf("usb: endpoint address %02X is not an IN endpoint", e.Address)
	}

	// Check if it's a Bulk endpoint
	if e.TransferType != TransferTypeBulk {
		return 0, fmt.Errorf("usb: endpoint address %02X is not a bulk endpoint (type %02X)", e.Address, e.TransferType)
	}

	bt := gusb.BulkTransfer{
		Ep:      uint32(e.Address), // Endpoint address including direction
		Len:     uint32(len(buffer)),
		Timeout: uint32(timeoutMs),
		Data:    gusb.SlicePtr(buffer),
	}

	n, err := gusb.Ioctl(e.i.d.f, gusb.USBDEVFS_BULK, &bt)
	if err != nil {
		return n, fmt.Errorf("usb: BulkIn from ep %02X failed: %w", e.Address, err)
	}
	return n, nil
}

func (e *OutEndpoint) WriteContext(ctx context.Context, buf []byte) (int, error) {
	// Check if the context is already cancelled
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		// Continue if the context is not cancelled
	}

	// Check if the device is open
	if e.i == nil || e.i.d == nil || e.i.d.f == nil {
		return 0, errors.New("usb: device not open for WriteContext")
	}

	// Check if it's an OUT endpoint (bit 7 of address is 0)
	if (e.Address & 0x80) != 0 {
		return 0, fmt.Errorf("usb: endpoint address %02X is not an OUT endpoint", e.Address)
	}

	// Check if it's a Bulk endpoint
	if e.TransferType != TransferTypeBulk {
		return 0, fmt.Errorf("usb: endpoint address %02X is not a bulk endpoint (type %02X)", e.Address, e.TransferType)
	}

	// Create a channel to receive the result from the goroutine
	resultChan := make(chan transferResult)

	// Launch a goroutine to perform the blocking BulkOut operation
	go func() {
		n, err := e.BulkOut(buf, 0) // Use a timeout of 0 for non-blocking operation
		resultChan <- transferResult{n, err}
	}()

	// Wait for either the context to be cancelled or the transfer to complete
	select {
	case <-ctx.Done():
		// Context cancelled, return the context error
		return 0, ctx.Err()
	case result := <-resultChan:
		// Transfer completed, return the result
		return result.n, result.err
	}
}

type transferResult struct {
	n   int
	err error
}

func (e *InEndpoint) ReadContext(ctx context.Context, buf []byte) (int, error) {
	// Check if the context is already cancelled
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
		// Continue if the context is not cancelled
	}

	// Check if the device is open
	if e.i == nil || e.i.d == nil || e.i.d.f == nil {
		return 0, errors.New("usb: device not open for ReadContext")
	}

	// Check if it's an IN endpoint (bit 7 of address is 1)
	if (e.Address & 0x80) == 0 {
		return 0, fmt.Errorf("usb: endpoint address %02X is not an IN endpoint", e.Address)
	}

	// Check if it's a Bulk endpoint
	if e.TransferType != TransferTypeBulk {
		return 0, fmt.Errorf("usb: endpoint address %02X is not a bulk endpoint (type %02X)", e.Address, e.TransferType)
	}

	// Create a channel to receive the result from the goroutine
	resultChan := make(chan transferResult)

	// Launch a goroutine to perform the blocking BulkIn operation
	go func() {
		n, err := e.BulkIn(buf, 0) // Use a timeout of 0 for non-blocking operation
		resultChan <- transferResult{n, err}
	}()

	// Wait for either the context to be cancelled or the transfer to complete
	select {
	case <-ctx.Done():
		// Context cancelled, return the context error
		return 0, ctx.Err()
	case result := <-resultChan:
		// Transfer completed, return the result
		return result.n, result.err
	}
}

func (e *Endpoint) Bulk() {
	// @todo: This might be a generic bulk transfer or could be deprecated by BulkIn/BulkOut
}

func (e *Endpoint) Interrupt() {
	// @todo: Implement interrupt transfer
}
