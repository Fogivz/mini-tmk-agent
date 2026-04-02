package audio

import (
	"fmt"
	"math"
	"os"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gordonklaus/portaudio"
)

var threshold = 100.0

func HasSpeech(samples []int16, threshold float64) bool {
	var sum float64

	for _, s := range samples {
		sum += math.Abs(float64(s))
	}

	avg := sum / float64(len(samples))

	return avg > threshold
}

func RecordWav(filePath string, seconds int) error {
	portaudio.Initialize()
	defer portaudio.Terminate()

	sampleRate := 16000
	buffer := make([]int16, sampleRate*seconds)

	stream, err := portaudio.OpenDefaultStream(
		1,
		0,
		float64(sampleRate),
		len(buffer),
		buffer,
	)
	if err != nil {
		return err
	}
	defer stream.Close()

	if err := stream.Start(); err != nil {
		return err
	}

	if err := stream.Read(); err != nil {
		return err
	}

	if !HasSpeech(buffer, threshold) {
		return fmt.Errorf("no speech detected")
	}

	if err := stream.Stop(); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := wav.NewEncoder(
		file,
		sampleRate,
		16,
		1,
		1,
	)

	intBuffer := &audio.IntBuffer{
		Format: &audio.Format{
			NumChannels: 1,
			SampleRate:  sampleRate,
		},
		Data:           make([]int, len(buffer)),
		SourceBitDepth: 16,
	}

	for i, sample := range buffer {
		intBuffer.Data[i] = int(sample)
	}

	if err := encoder.Write(intBuffer); err != nil {
		return err
	}

	return encoder.Close()
}
