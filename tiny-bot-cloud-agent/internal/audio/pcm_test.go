package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBytesToPCM16(t *testing.T) {
	cases := map[string]struct {
		b64     string
		wantLen int
		wantErr bool
	}{
		"empty":  {"", 0, false},
		"normal": {"AQIDBA==", 4, false}, // 0x01 0x02 0x03 0x04
		"odd":    {"AQ==", 0, true},      // base64 of 0x01, but binary len 1 → odd
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := BytesToPCM16(c.b64)
			if c.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, out, c.wantLen)
		})
	}
}

func TestPCM16ToBytes_RoundTrip(t *testing.T) {
	raw := []byte{0x01, 0x02, 0x03, 0x04}
	b64 := PCM16ToBytes(raw)
	assert.Equal(t, "AQIDBA==", b64)
	back, err := BytesToPCM16(b64)
	require.NoError(t, err)
	assert.Equal(t, raw, back)
}

func TestHeader_DurationAndFrame(t *testing.T) {
	h := DefaultHeader()
	assert.InDelta(t, 1.0, h.DurationSeconds(16000*2), 1e-6)
	assert.Equal(t, 1600, h.FrameBytes(50))   // 50ms @ 16k 16bit mono
	assert.Equal(t, 32000, h.FrameBytes(1000)) // 1s
}
