package audio

import (
	"encoding/binary"
	"fmt"
	"io"
)

// WriteWAV 把 PCM16 little-endian 裸流写成标准 WAV 文件，便于本地播放器直接打开。
func WriteWAV(w io.Writer, pcm []byte, sampleRate, channels, bitsPerSample int) error {
	if len(pcm)%2 != 0 {
		return fmt.Errorf("audio: wav pcm byte length %d not even", len(pcm))
	}
	if sampleRate <= 0 || channels <= 0 || bitsPerSample <= 0 {
		return fmt.Errorf("audio: invalid wav params rate=%d channels=%d bits=%d", sampleRate, channels, bitsPerSample)
	}

	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := uint32(len(pcm))
	chunkSize := 36 + dataSize

	header := make([]byte, 44)
	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], chunkSize)
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16) // PCM fmt chunk size
	binary.LittleEndian.PutUint16(header[20:22], 1) // audio format PCM
	binary.LittleEndian.PutUint16(header[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(header[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(header[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(header[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(header[34:36], uint16(bitsPerSample))
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], dataSize)

	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(pcm)
	return err
}
