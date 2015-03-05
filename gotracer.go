// Copyright (c) 2015, Roland Bali (roland.bali@spagettikod.se), Spagettikod
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without modification,
// are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
//    list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice, this
//    list of conditions and the following disclaimer in the documentation and/or
//    other materials provided with the distribution.
//
// 3. Neither the name of the copyright holder nor the names of its contributors may
//    be used to endorse or promote products derived from this software without
//    specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
// IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
// INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT
// NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
// PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY,
// WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

/*

	Package to communicate with the EPsolar Tracer BN-series solar charge controllers.

	It has been tested with the EPsolar Tracer4215BN but should work with all Tracer*BN
	series of controllers. The EPsolar provided RJ45 to USB cable can be used to connect
	the Tracer to a computer.

*/

package gotracer

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jacobsa/go-serial/serial"
)

// Status information read from Tracer
type TracerStatus struct {
	ArrayVoltage           float32   `json:"pvv"`     // Solar panel voltage, (V)
	ArrayCurrent           float32   `json:"pvc"`     // Solar panel current, (A)
	ArrayPower             float32   `json:"pvp"`     // Solar panel power, (W)
	BatteryVoltage         float32   `json:"bv"`      // Battery voltage, (V)
	BatteryCurrent         float32   `json:"bc"`      // Battery current, (A)
	BatterySOC             int32     `json:"bsoc"`    // Battery state of charge, (%)
	BatteryTemp            float32   `json:"btemp"`   // Battery temperatur, (C)
	BatteryMaxVoltage      float32   `json:"bmaxv"`   // Battery maximum voltage, (V)
	BatteryMinVoltage      float32   `json:"bminv"`   // Battery lowest voltage, (V)
	DeviceTemp             float32   `json:"devtemp"` // Tracer temperature, (C)
	LoadVoltage            float32   `json:"lv"`      // Load voltage, (V)
	LoadCurrent            float32   `json:"lc"`      // Load current, (A)
	LoadPower              float32   `json:"lp"`      // Load power, (W)
	Load                   bool      `json:"load"`    // Shows whether load is on or off
	EnergyConsumedDaily    float32   `json:"ecd"`     // Tracer calculated daily consumption, (kWh)
	EnergyConsumedMonthly  float32   `json:"ecm"`     // Tracer calculated monthly consumption, (kWh)
	EnergyConsumedAnnual   float32   `json:"eca"`     // Tracer calculated annual consumption, (kWh)
	EnergyConsumedTotal    float32   `json:"ect"`     // Tracer calculated total consumption, (kWh)
	EnergyGeneratedDaily   float32   `json:"egd"`     // Tracer calculated daily power generation, (kWh)
	EnergyGeneratedMonthly float32   `json:"egm"`     // Tracer calculated monthly power generation, (kWh)
	EnergyGeneratedAnnual  float32   `json:"ega"`     // Tracer calculated annual power generation, (kWh)
	EnergyGeneratedTotal   float32   `json:"egt"`     // Tracer calculated total power generation, (kWh)
	Timestamp              time.Time `json:"t"`
}

// Formatted output showing all status parameters
func (t TracerStatus) String() string {
	return fmt.Sprintf("ArrayVoltage: %.2f\nArrayCurrent: %.2f\nArrayPower: %.2f\nBatteryVoltage: %.2f\nBatteryCurrent: %.2f\nBatterySOC: %v%%\nBatteryTemp: %.2f\nBatteryMaxVoltage: %.2f\nBatteryMinVoltage: %.2f\nDeviceTemp: %.2f\nLoadVoltage: %.2f\nLoadCurrent: %.2f\nLoadPower: %.2f\nLoad: %t\nEnergyConsumedDaily: %.2f\nEnergyConsumedMonthly: %.2f\nEnergyConsumedAnnual:%.2f\nEnergyConsumedTotal:%.2f\nEnergyGeneratedDaily: %.2f\nEnergyGeneratedMonthly: %.2f\nEnergyGeneratedAnnual: %.2f\nEnergyGeneratedTotal: %.2f\n", t.ArrayVoltage, t.ArrayCurrent, t.ArrayPower, t.BatteryVoltage, t.BatteryCurrent, t.BatterySOC, t.BatteryTemp, t.BatteryMaxVoltage, t.BatteryMinVoltage, t.DeviceTemp, t.LoadVoltage, t.LoadCurrent, t.LoadPower, t.Load, t.EnergyConsumedDaily, t.EnergyConsumedMonthly, t.EnergyConsumedAnnual, t.EnergyConsumedTotal, t.EnergyGeneratedDaily, t.EnergyGeneratedMonthly, t.EnergyGeneratedAnnual, t.EnergyGeneratedTotal)
}

type command struct {
	data    []byte
	respLen int
	offset  int
}

var (
	queryStateCommand []command = []command{{data: []byte{0x01, 0x04, 0x32, 0x00, 0x00, 0x03, 0xbe, 0xb3}, respLen: 11, offset: 0},
		{data: []byte{0x01, 0x02, 0x20, 0x00, 0x00, 0x01, 0xb2, 0x0a}, respLen: 6, offset: 11},
		{data: []byte{0x01, 0x43, 0x31, 0x00, 0x00, 0x1b, 0x0a, 0xf2}, respLen: 51, offset: 17},
		{data: []byte{0x01, 0x04, 0x33, 0x1a, 0x00, 0x03, 0x9e, 0x88}, respLen: 11, offset: 68},
		{data: []byte{0x01, 0x04, 0x33, 0x02, 0x00, 0x12, 0xde, 0x83}, respLen: 41, offset: 79}}
)

// Read status information from the Tracer connected on specified portName.
func Status(portName string) (t TracerStatus, err error) {
	options := serial.OpenOptions{
		PortName:        portName,
		BaudRate:        115200,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 4,
	}

	var port io.ReadWriteCloser
	port, err = serial.Open(options)
	if err != nil {
		return
	}
	defer port.Close()

	var buffer []byte = make([]byte, 120)
	for _, r := range queryStateCommand {
		if _, err = port.Write(r.data); err != nil {
			return
		}
		var b []byte
		if b, err = readWithTimeout(port, r.respLen); err != nil {
			return
		}
		copy(buffer[r.offset:], b)
	}

	t.Timestamp = time.Now().UTC()

	t.Load = int(buffer[8]) == 1
	t.ArrayVoltage = unpack(buffer[24:26]) / 100
	t.ArrayCurrent = unpack(buffer[26:28]) / 100
	t.ArrayPower = unpack(buffer[28:30]) / 100
	t.BatteryVoltage = unpack(buffer[32:34]) / 100
	t.LoadVoltage = unpack(buffer[40:42]) / 100
	t.LoadCurrent = unpack(buffer[42:44]) / 100
	t.LoadPower = unpack(buffer[44:46]) / 100
	t.BatteryTemp = unpack(buffer[56:58]) / 100
	t.DeviceTemp = unpack(buffer[58:60]) / 100
	t.BatterySOC = int32(buffer[65])

	// Battery current can be negative.
	bc := unpack(buffer[73:75])
	if bc > 32768 {
		bc = bc - 65536
	}
	t.BatteryCurrent = bc / 100
	t.BatteryMaxVoltage = unpack(buffer[82:84]) / 100
	t.BatteryMinVoltage = unpack(buffer[84:86]) / 100
	t.EnergyConsumedDaily = unpack(buffer[86:88]) / 100
	t.EnergyConsumedMonthly = unpack(buffer[88:92]) / 100
	t.EnergyConsumedAnnual = unpack(buffer[92:96]) / 100
	t.EnergyConsumedTotal = unpack(buffer[96:100]) / 100
	t.EnergyGeneratedDaily = unpack(buffer[100:104]) / 100
	t.EnergyGeneratedMonthly = unpack(buffer[104:108]) / 100
	t.EnergyGeneratedAnnual = unpack(buffer[108:112]) / 100
	t.EnergyGeneratedTotal = unpack(buffer[112:116]) / 100

	return
}

func readWithTimeout(r io.Reader, n int) ([]byte, error) {
	buf := make([]byte, 120)
	done := make(chan error)
	readAndCallBack := func() {
		_, err := io.ReadAtLeast(r, buf, n)
		done <- err
	}

	go readAndCallBack()

	timeout := make(chan bool)
	sleepAndCallBack := func() { time.Sleep(2e9); timeout <- true }
	go sleepAndCallBack()

	select {
	case err := <-done:
		return buf, err
	case <-timeout:
		return nil, errors.New("Timed out.")
	}

	return nil, errors.New("Can't get here.")
}

// Converts a slice of bytes to a float. Byte values are shifted according
// to their locaiton in the slice. First item in the slice is the highest
// byte and the last one is the lowest.
func unpack(slice []byte) float32 {
	var v uint32
	for i, b := range slice {
		shift := uint(((len(slice) - 1 - i) * 8))
		v += uint32(b) << shift
	}
	return float32(v)
}
