package gusb

import (
	"log"
	"os"

	"golang.org/x/sys/unix"
)

func Claim(f *os.File, ifno int32) error {
	if r, errno := Ioctl(f, USBDEVFS_IOCTL, &IoctlPacket{
		IfNo:      ifno,
		IoctlCode: int32(USBDEVFS_DISCONNECT), // disconn kernel driver
		Data:      0,
	}); errno == unix.ENODATA {
		// Debug: no previous kernel driver attached
	} else if r == -1 {
		log.Printf("ERROR: driver disconnect failed: %d, %v\n", r, errno)
	}

	if r, errno := Ioctl(f, USBDEVFS_CLAIMINTERFACE, &ifno); r == -1 {
		return errno
	}
	return nil
}
func Release(f *os.File, ifno int32) error {
	if r, errno := Ioctl(f, USBDEVFS_RELEASEINTERFACE, &ifno); r == -1 {
		return errno
	}

	if r, errno := Ioctl(f, USBDEVFS_IOCTL, &IoctlPacket{
		IfNo:      ifno,
		IoctlCode: int32(USBDEVFS_CONNECT), //reconnect kernel driver
		Data:      0,
	}); r == -1 {
		log.Printf("ERROR: driver connect failed: %d, %v\n", r, errno)
	}
	return nil
}

func GetDriver(f *os.File, ifno int32) (string, error) {
	drv := GetDriverS{
		Interface: uint32(ifno),
	}

	_, err := Ioctl(f, USBDEVFS_GETDRIVER, &drv)
	if err == unix.ENODATA { // empty if nothing is in use
		// empty string?
	} else if err != nil {
		log.Printf("ERROR: Could not get driver: %v\n", err)
		return "", err
	}
	return string(drv.Driver[:]), nil
}

func GetSpeed(f *os.File) (DeviceSpeed, error) {
	r, err := Ioctl(f, USBDEVFS_GET_SPEED, nil)
	if err != nil {
		log.Printf("ERROR: Unable to get device speed: %v\n", err)
		return SpeedUnknown, err
	}
	return DeviceSpeed(r), nil
}

/*
func Ctl(f *os.File, d ...interface{}) (int, error) {
	if r, err := Ioctl(f, USBDEVFS_CONTROL, &CtrlTransfer{
		RequestType: uint8(d),
		Request:     uint8(d),
		Value:       uint16(d),
		Index:       uint16(d),
		Length:      uint16(len(d)),
		Timeout:     uint32(d),
		Data:        slicePtr(data),
	}); r == -1 {
		return -1, err
	} else {
		return r, nil
	}
}
*/
