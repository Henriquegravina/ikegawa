// Package typroto implements the Yokogawa 92015 communication package
// protocol used by the TY700/TY500-series digital multimeters
// (TY710, TY720, TY530). Reference: Yokogawa manual IM92015-02E.
package typroto

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
)

// readAttemptTimeout bounds each individual port read. Kept short so a
// cancellation (stop channel) or a silent instrument is noticed quickly
// instead of blocking for a long time.
const readAttemptTimeout = 1 * time.Second

// maxSilentAttempts is how many consecutive empty reads (no bytes at all)
// we tolerate before concluding the instrument isn't responding — e.g. it
// was powered off or the cable was unplugged. maxSilentAttempts *
// readAttemptTimeout is the worst-case time to notice this.
const maxSilentAttempts = 6

// ErrNoResponse means the instrument produced no data for a while — most
// likely it was powered off or physically disconnected.
var ErrNoResponse = errors.New("sem resposta do instrumento (desligado ou desconectado?)")

// ErrCancelled means a caller-supplied stop channel closed before a full
// line arrived.
var ErrCancelled = errors.New("leitura cancelada")

// Mode returns the fixed serial settings required by the instrument:
// 9600 bps, 8 data bits, no parity, 2 stop bits, no handshake.
func Mode() *serial.Mode {
	return &serial.Mode{
		BaudRate: 9600,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.TwoStopBits,
	}
}

// Reading is a single decoded measurement.
type Reading struct {
	Status   string // "Normal", "Over-range", "Low battery"
	Value    float64
	AuxUnit  string // e.g. "m", "k" (may be empty)
	Unit     string // e.g. "V", "A"
	Function string // e.g. "DC", "AC"
	Raw      string
}

func (r Reading) String() string {
	return fmt.Sprintf("%+g %s%s %s", r.Value, r.AuxUnit, r.Unit, r.Function)
}

// After the numeric value, the instrument always sends exactly 5 more
// characters before the CRLF: a 1-char auxiliary/multiplier prefix (k, m,
// n, M, ...), a 1-char unit, a 2-char function code, and a 1-char
// checksum. Any of those sub-fields is space-padded when not applicable
// rather than omitted, and the unit+function split doesn't always land on
// a "natural" word boundary — e.g. resistance ("OHM") comes across as
// unit 'O' + function "HM", and capacitance ("F", no DC/AC-style function)
// as unit 'F' + function "  ". Examples seen on a TY720:
//
//	"+00.213mVDC3"   -> aux 'm', unit 'V', func "DC"
//	"+0.0001 VDC1"   -> aux ' ', unit 'V', func "DC"
//	"+00.000MOHM5"   -> aux 'M', unit 'O', func "HM"
//	"+00.017nF  F"   -> aux 'n', unit 'F', func "  "
//	"+0021.9 C  2"   -> aux ' ', unit 'C', func "  "
//
// The checksum is treated as an opaque trailing byte (not assumed to be a
// hex digit): "." matches it whatever it is.
var measureRE = regexp.MustCompile(
	`^(?:RR|RS|RL),([NOB]),([+-])(\d+\.?\d*)(.)(.)(.)(.)(.)$`,
)

var statusNames = map[string]string{"N": "Normal", "O": "Over-range", "B": "Low battery"}

// ParseReading parses a "RR,..."/"RS,..."/"RL,..." measurement response line.
func ParseReading(line string) (Reading, error) {
	line = strings.TrimRight(line, "\r\n")
	m := measureRE.FindStringSubmatch(line)
	if m == nil {
		return Reading{}, fmt.Errorf("unrecognized response: %q", line)
	}
	value, err := strconv.ParseFloat(m[2]+m[3], 64)
	if err != nil {
		return Reading{}, fmt.Errorf("bad value in %q: %w", line, err)
	}
	return Reading{
		Status:   statusNames[m[1]],
		Value:    value,
		AuxUnit:  strings.TrimSpace(m[4]),
		Unit:     strings.TrimSpace(m[5]),
		Function: strings.TrimSpace(m[6] + m[7]),
		Raw:      line,
	}, nil
}

// Client drives a Yokogawa DMM over an already-opened serial port.
type Client struct {
	port    serial.Port
	pending []byte // bytes already read from the port but not yet consumed as a line

	// mu serializes access to the serial port: two goroutines reading/
	// writing it at once could interleave their bytes. Every method below
	// holds it for the duration of its own send/receive.
	//
	// This is a safety net, not a substitute for correct UI flow: the
	// instrument's protocol itself can't interleave a continuous stream
	// (RR,2) with one-shot commands (RR,1, RS, RL, ...), so callers must
	// still stop streaming before issuing any other command.
	mu sync.Mutex
}

func NewClient(port serial.Port) *Client {
	_ = port.SetReadTimeout(readAttemptTimeout)
	return &Client{port: port}
}

func (c *Client) send(cmd string) error {
	_, err := c.port.Write([]byte(cmd + "\r\n"))
	return err
}

// readLine reads one CRLF-terminated line, discarding empty/garbage lines
// (the adapter occasionally emits stray bytes right after a mode switch).
//
// It does its own buffering directly on top of the port instead of
// bufio.Reader: go.bug.st/serial's Read returns (0, nil) on a timeout
// rather than an error, and bufio retries that internally up to 100 times
// before giving up — worth several minutes stuck with the port's mutex
// held. Here every attempt is bounded by readAttemptTimeout, we give up
// after maxSilentAttempts consecutive empty ones (ErrNoResponse), and stop,
// if non-nil, is checked before each attempt so a caller can cancel a
// streaming read within about one attempt's timeout.
func (c *Client) readLine(stop <-chan struct{}) (string, error) {
	buf := make([]byte, 256)
	silent := 0
	for {
		if idx := bytes.IndexByte(c.pending, '\n'); idx >= 0 {
			line := strings.TrimRight(string(c.pending[:idx]), "\r\n")
			c.pending = c.pending[idx+1:]
			if line == "" {
				continue
			}
			return line, nil
		}
		if stop != nil {
			select {
			case <-stop:
				return "", ErrCancelled
			default:
			}
		}
		n, err := c.port.Read(buf)
		if err != nil {
			return "", err
		}
		if n == 0 {
			silent++
			if silent >= maxSilentAttempts {
				return "", ErrNoResponse
			}
			continue
		}
		silent = 0
		c.pending = append(c.pending, buf[:n]...)
	}
}

// ReadLine reads one raw line, holding the port lock only for this single
// read. Used by the streaming loop, which calls it once per incoming line
// and can pass a stop channel to cancel promptly.
func (c *Client) ReadLine(stop <-chan struct{}) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readLine(stop)
}

// SingleReading requests and parses one live measurement (RR,1).
func (c *Client) SingleReading() (Reading, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send("RR,1"); err != nil {
		return Reading{}, err
	}
	line, err := c.readLine(nil)
	if err != nil {
		return Reading{}, err
	}
	return ParseReading(line)
}

// StartStream begins continuous live measurements (RR,2). Read results
// with ReadLine/ParseReading in a loop until StopStream is called.
func (c *Client) StartStream() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.send("RR,2")
}

// StopStream halts continuous live measurements (RR,0).
func (c *Client) StopStream() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.send("RR,0")
}

// ErrEmptySlot indicates the requested memory slot has no stored data.
var ErrEmptySlot = fmt.Errorf("memory slot is empty")

// ReadSaveSlot fetches save-mode memory slot n (0-99): RS,nn.
func (c *Client) ReadSaveSlot(n int) (Reading, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send(fmt.Sprintf("RS,%02d", n)); err != nil {
		return Reading{}, err
	}
	line, err := c.readLine(nil)
	if err != nil {
		return Reading{}, err
	}
	if strings.HasPrefix(line, "RS5") {
		return Reading{}, ErrEmptySlot
	}
	return ParseReading(line)
}

// ReadLoggingSlot fetches logging-mode memory slot n: RL,nnnn.
func (c *Client) ReadLoggingSlot(n int) (Reading, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send(fmt.Sprintf("RL,%04d", n)); err != nil {
		return Reading{}, err
	}
	line, err := c.readLine(nil)
	if err != nil {
		return Reading{}, err
	}
	if strings.HasPrefix(line, "RLE") {
		return Reading{}, ErrEmptySlot
	}
	return ParseReading(line)
}

var loggingCountRE = regexp.MustCompile(`^RN,(\d{3,5})([0-9A-Fa-f])$`)

// LoggingCount returns how many records are stored in logging-mode memory (RN).
func (c *Client) LoggingCount() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send("RN"); err != nil {
		return 0, err
	}
	line, err := c.readLine(nil)
	if err != nil {
		return 0, err
	}
	m := loggingCountRE.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return 0, fmt.Errorf("unrecognized RN response: %q", line)
	}
	return strconv.Atoi(m[1])
}

var loggingIntervalRE = regexp.MustCompile(`^RI,(\d{4})([0-9A-Fa-f])$`)

// LoggingInterval returns the logging-mode sampling interval, in seconds (RI).
func (c *Client) LoggingInterval() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.send("RI"); err != nil {
		return 0, err
	}
	line, err := c.readLine(nil)
	if err != nil {
		return 0, err
	}
	m := loggingIntervalRE.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return 0, fmt.Errorf("unrecognized RI response: %q", line)
	}
	return strconv.Atoi(m[1])
}

// Close closes the underlying serial port.
func (c *Client) Close() error { return c.port.Close() }

// ListPorts returns the available serial port device names.
func ListPorts() ([]string, error) {
	return serial.GetPortsList()
}

var _ io.Closer = (*Client)(nil)
