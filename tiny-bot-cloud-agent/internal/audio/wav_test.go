package audio

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteWAV(t *testing.T) {
	pcm := []byte{0x00, 0x01, 0xff, 0x7f}
	var buf bytes.Buffer
	require.NoError(t, WriteWAV(&buf, pcm, 16000, 1, 16))

	b := buf.Bytes()
	assert.Equal(t, "RIFF", string(b[0:4]))
	assert.Equal(t, "WAVE", string(b[8:12]))
	assert.Equal(t, uint32(40), binary.LittleEndian.Uint32(b[4:8]))
	assert.Equal(t, uint16(1), binary.LittleEndian.Uint16(b[20:22]))
	assert.Equal(t, uint32(16000), binary.LittleEndian.Uint32(b[24:28]))
	assert.Equal(t, "data", string(b[36:40]))
	assert.Equal(t, uint32(4), binary.LittleEndian.Uint32(b[40:44]))
	assert.Equal(t, pcm, b[44:])
}
