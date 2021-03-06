package max

import (
	"fmt"
	"testing"

	"github.com/advancedclimatesystems/io/dac"
	"github.com/advancedclimatesystems/io/iotest"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/io/i2c"
)

func TestDACinterface(t *testing.T) {
	assert.Implements(t, (*dac.DAC)(nil), new(MAX5813))
	assert.Implements(t, (*dac.DAC)(nil), new(MAX5814))
	assert.Implements(t, (*dac.DAC)(nil), new(MAX5815))
}

func TestNewMAX581x(t *testing.T) {
	conn, _ := i2c.Open(iotest.NewI2CDriver(iotest.NewI2CConn()), 0x1)

	max5813, _ := NewMAX5813(conn, 3)
	assert.Equal(t, 8, max5813.resolution)

	max5814, _ := NewMAX5814(conn, 3)
	assert.Equal(t, 10, max5814.resolution)

	max5815, _ := NewMAX5815(conn, 3)
	assert.Equal(t, 12, max5815.resolution)
}

func TestMAX581xSetVref(t *testing.T) {
	data := make(chan []byte, 2)
	c := iotest.NewI2CConn()
	c.TxFunc(func(w, _ []byte) error {
		data <- w
		return nil
	})

	conn, _ := i2c.Open(iotest.NewI2CDriver(c), 0x1)

	m := max581x{
		conn:       conn,
		resolution: 8,
	}

	var tests = []struct {
		vref     float64
		expected []byte
	}{
		{2.5, []byte{0x75, 0, 0}},
		{2.048, []byte{0x76, 0, 0}},
		{4.096, []byte{0x77, 0, 0}},
	}

	for _, test := range tests {
		m.SetVref(test.vref)
		assert.Equal(t, test.expected, <-data)
	}

}

func TestMAX581xSetVoltage(t *testing.T) {
	data := make(chan []byte, 2)
	c := iotest.NewI2CConn()
	c.TxFunc(func(w, _ []byte) error {
		data <- w
		return nil
	})

	conn, _ := i2c.Open(iotest.NewI2CDriver(c), 0x1)
	m := max581x{
		conn: conn,
	}

	var tests = []struct {
		resolution int
		vref       float64
		voltage    float64
		channel    int
		expected   []byte
	}{
		{8, 2.5, 2.5, 1, []byte{0x31, 0xff, 0}},
		{8, 5, 2.5, 2, []byte{0x32, 0x7f, 0}},
		{8, 5, 0, 2, []byte{0x32, 0, 0}},

		{10, 5, 5, 2, []byte{0x32, 0xff, 0xc0}},
		{10, 5, 2.5, 2, []byte{0x32, 0x7f, 0xc0}},
		{10, 5, 0, 2, []byte{0x32, 0, 0}},

		{12, 2.5, 2.5, 3, []byte{0x33, 0xff, 0xf0}},
		{12, 5, 2.5, 3, []byte{0x33, 0x7f, 0xf0}},
		{12, 10, 2, 3, []byte{0x33, 0x33, 0x30}},
	}

	for _, test := range tests {
		m.resolution = test.resolution
		m.vref = test.vref

		err := m.SetVoltage(test.voltage, test.channel)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, test.expected, <-data)

	}
}

func TestMAX581xConn(t *testing.T) {
	c, _ := i2c.Open(iotest.NewI2CDriver(iotest.NewI2CConn()), 0x1)
	dac, _ := NewMAX5813(c, 2.048)
	assert.Equal(t, c, dac.Conn())
}

// TestMAX581xSetInputCodeWithInvalidChannel calls dac.SetInputCode with a
// input code that lies outside the range of the DAC.
func TestMAX581xSetInputCodeWithInvalidCode(t *testing.T) {
	var tests = []struct {
		dac  dac.DAC
		code int
	}{
		{MAX5813{}, -1},
		{MAX5813{}, 256},
		{MAX5814{}, -1},
		{MAX5814{}, 1024},
		{MAX5815{}, -1},
		{MAX5815{}, 4096},
	}

	for _, test := range tests {
		assert.EqualError(
			t,
			test.dac.SetInputCode(test.code, 1),
			fmt.Sprintf("digital input code %d is out of range of 0 <= code < 1", test.code))
	}
}

// TestMAX581xSetInputCodeWithInvalidChannel calls dac.SetInputCode with a
// channel that isn't in the range of the DAC.
func TestMAX581xSetInputCodeWithInvalidChannel(t *testing.T) {
	dac := max581x{}

	for _, channel := range []int{-1, 4} {
		assert.EqualError(
			t,
			dac.SetInputCode(512, channel),
			fmt.Sprintf("%d is not a valid channel", channel))
	}
}

// TestMAX581xWithFailingConnection test if all DAC's return errors when the
// connection fails.
func TestMAX581xWithFailingConnection(t *testing.T) {
	c := iotest.NewI2CConn()
	c.TxFunc(func(w, _ []byte) error {
		return fmt.Errorf("som error occured")
	})
	conn, _ := i2c.Open(iotest.NewI2CDriver(c), 0x1)

	_, err := NewMAX5813(conn, 2.048)
	assert.NotNil(t, err)

	_, err = NewMAX5814(conn, 2.048)
	assert.NotNil(t, err)

	_, err = NewMAX5815(conn, 2.048)
	assert.NotNil(t, err)

	dac := max581x{
		conn:       conn,
		vref:       2.048,
		resolution: 8,
	}

	assert.NotNil(t, dac.SetInputCode(512, 1))
}

func ExampleMAX5813() {
	d, err := i2c.Open(&i2c.Devfs{
		Dev: "/dev/i2c-0",
	}, 0x1c)

	if err != nil {
		panic(fmt.Sprintf("failed to open device: %v", err))
	}
	defer d.Close()

	// 2.5V is the input reference of the DAC.
	dac, err := NewMAX5813(d, 2.5)

	if err != nil {
		panic(fmt.Sprintf("failed to create MAX5813: %v", err))
	}

	// Set output of channel 1 to 1.3V.
	if err := dac.SetVoltage(1.3, 1); err != nil {
		panic(fmt.Sprintf("failed to set voltage: %v", err))
	}

	// It's also possible to set output of a channel with digital output code.
	if err := dac.SetInputCode(128, 1); err != nil {
		panic(fmt.Sprintf("failed to set voltage using output code: %v", err))
	}
}
