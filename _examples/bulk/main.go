package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pzl/usb"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Arguments required: <vid> <pid> <hex of bytearray>")
		os.Exit(1)
	}
	vid, err := strconv.ParseUint(strings.TrimPrefix(os.Args[1], "0x"), 16, 16)
	if err != nil {
		panic(err)
	}
	pid, err := strconv.ParseUint(strings.TrimPrefix(os.Args[2], "0x"), 16, 16)
	if err != nil {
		panic(err)
	}

	fmt.Printf("looking for %04x:%04x\n", vid, pid)

	ctx := usb.NewContext()
	dev, err := ctx.OpenDeviceWithVIDPID(usb.ID(vid), usb.ID(pid))
	if err == usb.ErrDeviceNotFound {
		fmt.Println("Device Not found")
		return
	} else if err != nil {
		panic(err)
	}

	iface, done, err := dev.DefaultInterface()
	if err != nil {
		panic(err)
	}
	defer done()

	out, err := iface.GetOutEndpoint()
	if err != nil {
		panic(err)
	}

	in, err := iface.GetInEndpoint()
	if err != nil {
		panic(err)
	}

	byteArrayString := os.Args[3]
	hexByteArray, err := hex.DecodeString(byteArrayString)
	if err != nil {
		panic(err)
	}

	bytesWritten, err := out.WriteContext(ctx, hexByteArray)

	if err != nil {
		panic(err)
	}
	fmt.Printf("Wrote %v bytes\n", bytesWritten)
	rawRsp := make([]byte, 1024)
	bytesRead, err := in.ReadContext(ctx, rawRsp)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Read %v bytes\n", bytesRead)
	fmt.Printf("Response: %v\n", rawRsp)
}
