//go:build linux

package protocol

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	bugserial "go.bug.st/serial"
)

type SerialHeadset struct {
	port  bugserial.Port
	txID  uint16
	rxbuf []byte
}

type SerialPortInfo struct {
	Path        string
	TTY         string
	Interface   string
	Description string
}

func SerialPortsList() ([]string, error) {
	return bugserial.GetPortsList()
}

func FindSonySerialPort(vid uint16, pid uint16) (string, error) {
	ports, err := FindSonySerialPorts(vid, pid)
	if err != nil {
		return "", err
	}
	if len(ports) == 0 {
		return "", fmt.Errorf("no serial tty found for VID:PID %04x:%04x", vid, pid)
	}
	return ports[0].Path, nil
}

func FindSonySerialPorts(vid uint16, pid uint16) ([]SerialPortInfo, error) {
	entries, err := filepath.Glob("/sys/class/tty/*")
	if err != nil {
		return nil, err
	}
	wantVID := fmt.Sprintf("%04x", vid)
	wantPID := fmt.Sprintf("%04x", pid)
	ports := make([]SerialPortInfo, 0)
	for _, entry := range entries {
		name := filepath.Base(entry)
		if strings.HasPrefix(name, "ttyS") {
			continue
		}
		device, err := filepath.EvalSymlinks(filepath.Join(entry, "device"))
		if err != nil {
			continue
		}
		for dir := device; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
			idVendor, _ := os.ReadFile(filepath.Join(dir, "idVendor"))
			idProduct, _ := os.ReadFile(filepath.Join(dir, "idProduct"))
			if strings.EqualFold(strings.TrimSpace(string(idVendor)), wantVID) &&
				strings.EqualFold(strings.TrimSpace(string(idProduct)), wantPID) {
				ports = append(ports, SerialPortInfo{
					Path:        filepath.Join("/dev", name),
					TTY:         name,
					Interface:   serialInterfaceNumber(device),
					Description: serialInterfaceDescription(device),
				})
				break
			}
		}
	}
	sortSerialPorts(ports)
	return ports, nil
}

func serialInterfaceNumber(device string) string {
	for dir := device; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if b, err := os.ReadFile(filepath.Join(dir, "bInterfaceNumber")); err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return ""
}

func serialInterfaceDescription(device string) string {
	for dir := device; dir != "/" && dir != "."; dir = filepath.Dir(dir) {
		if b, err := os.ReadFile(filepath.Join(dir, "interface")); err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return ""
}

func sortSerialPorts(ports []SerialPortInfo) {
	score := func(p SerialPortInfo) int {
		score := 0
		if strings.Contains(strings.ToLower(p.Description), "vcom") {
			score -= 100
		}
		switch p.Interface {
		case "07":
			score -= 50
		case "06":
			score -= 40
		}
		if strings.HasPrefix(p.TTY, "ttyACM") {
			score -= 10
		}
		return score
	}
	for i := 0; i < len(ports); i++ {
		for j := i + 1; j < len(ports); j++ {
			if score(ports[j]) < score(ports[i]) {
				ports[i], ports[j] = ports[j], ports[i]
			}
		}
	}
}

func OpenSerialHeadset(path string) (*SerialHeadset, error) {
	return OpenSerialHeadsetWithBaud(path, 115200)
}

func OpenSerialHeadsetWithBaud(path string, baud int) (*SerialHeadset, error) {
	port, err := bugserial.Open(path, &bugserial.Mode{
		BaudRate: baud,
		DataBits: 8,
		Parity:   bugserial.NoParity,
		StopBits: bugserial.OneStopBit,
		InitialStatusBits: &bugserial.ModemOutputBits{
			RTS: true,
			DTR: true,
		},
	})
	if err != nil {
		return nil, err
	}
	_ = port.SetRTS(true)
	_ = port.SetDTR(true)
	_ = port.SetReadTimeout(50 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	_ = port.ResetInputBuffer()
	_ = port.ResetOutputBuffer()
	return &SerialHeadset{port: port}, nil
}

func (s *SerialHeadset) Close() error {
	if s == nil || s.port == nil {
		return nil
	}
	return s.port.Close()
}

func (s *SerialHeadset) WritePacket(pkt []byte) error {
	_, err := s.port.Write(pkt)
	return err
}

func (s *SerialHeadset) SetControlLines(rts bool, dtr bool) error {
	if err := s.port.SetRTS(rts); err != nil {
		return err
	}
	if err := s.port.SetDTR(dtr); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	return nil
}

func (s *SerialHeadset) ModemStatusBits() (*bugserial.ModemStatusBits, error) {
	return s.port.GetModemStatusBits()
}

func (s *SerialHeadset) nextTxID() uint16 {
	s.txID++
	return s.txID
}

func serialEventAddress(eventID byte) byte {
	switch eventID {
	case Evt2GHZConnectStatus, 9:
		return byte((AddrTX << 4) | AddrPC)
	default:
		return byte((AddrRX << 4) | AddrPC)
	}
}

func (s *SerialHeadset) writeGet(eventID byte) error {
	return s.WritePacket(BuildHciGetWithAddress(eventID, serialEventAddress(eventID), s.nextTxID(), nil))
}

func (s *SerialHeadset) writeSet(eventID byte, param []byte) error {
	return s.WritePacket(BuildHciSetWithAddress(eventID, serialEventAddress(eventID), s.nextTxID(), param))
}

func (s *SerialHeadset) ReadEvent(timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	tmp := make([]byte, 64)
	for time.Now().Before(deadline) {
		if event := s.popEvent(); event != nil {
			return event, nil
		}
		n, err := s.port.Read(tmp)
		if err != nil {
			if err == io.EOF {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			return nil, err
		}
		if n == 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		s.rxbuf = append(s.rxbuf, tmp[:n]...)
	}
	return nil, fmt.Errorf("read timeout (no serial response from device)")
}

func (s *SerialHeadset) popEvent() []byte {
	for i := 0; i+3 < len(s.rxbuf); i++ {
		if s.rxbuf[i] != HciEvtPacketType || s.rxbuf[i+1] != 0xff {
			continue
		}
		total := int(s.rxbuf[i+2]) + 3
		if total <= 0 {
			continue
		}
		if i+total > len(s.rxbuf) {
			if i > 0 {
				s.rxbuf = s.rxbuf[i:]
			}
			return nil
		}
		out := make([]byte, total)
		copy(out, s.rxbuf[i:i+total])
		s.rxbuf = s.rxbuf[i+total:]
		return out
	}
	if len(s.rxbuf) > 256 {
		s.rxbuf = s.rxbuf[len(s.rxbuf)-16:]
	}
	return nil
}

func (s *SerialHeadset) ReadRaw(timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 0, 128)
	tmp := make([]byte, 64)
	for time.Now().Before(deadline) {
		n, err := s.port.Read(tmp)
		if err != nil {
			if err == io.EOF {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			return nil, err
		}
		if n == 0 {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		buf = append(buf, tmp[:n]...)
		time.Sleep(20 * time.Millisecond)
	}
	if len(buf) == 0 {
		return nil, fmt.Errorf("read timeout (no serial bytes from device)")
	}
	return buf, nil
}

func SerialDebugGetPacket(eventID byte, txID uint16) []byte {
	return BuildHciGetWithAddress(eventID, serialEventAddress(eventID), txID, nil)
}

func GetSerialHeadsetInfo(s *SerialHeadset) (model *HeadsetModelInfo, batt *BatteryInfo, fw []byte, err error) {
	if err = s.writeGet(Evt2GHZConnectStatus); err != nil {
		return
	}
	_, _ = s.ReadEvent(hciReadTimeout)

	if pkt, e := QuerySerialHciEvent(s, EvtModelInfo); e == nil {
		model, _ = ParseModelInfo(pkt.Param)
	}

	if pkt, e := QuerySerialHciEvent(s, EvtBatteryInfo); e == nil {
		batt, _ = ParseBatteryInfo(pkt.Param)
	}

	if pkt, e := QuerySerialHciEvent(s, EvtFWVersion); e == nil {
		fw = pkt.Param
	}
	err = nil
	return
}

func QuerySerialHciEvent(s *SerialHeadset, eventID byte) (*HciPacket, error) {
	if err := s.writeGet(eventID); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(hciReadTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		r, err := s.ReadEvent(time.Until(deadline))
		if err != nil {
			return nil, err
		}
		ev, err := ParseHciEvent(r)
		if err != nil {
			lastErr = err
			continue
		}
		if ev.EventID == eventID {
			return ev, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("read timeout waiting for serial event %d", eventID)
}

func SendSerialHciSet(s *SerialHeadset, eventID byte, param []byte) error {
	if err := s.writeSet(eventID, param); err != nil {
		return err
	}
	_, _ = s.ReadEvent(hciReadTimeout)
	return nil
}
