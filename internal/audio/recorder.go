package audio

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/go-audio/audio"
	"github.com/go-audio/wav"
	"github.com/gordonklaus/portaudio"
)

var threshold = loadThreshold()
var recordDuration = loadRecordDuration()

func loadThreshold() float64 {
	v := strings.TrimSpace(os.Getenv("MINI_TMK_VAD_THRESHOLD"))
	if v == "" {
		// Default lower than before to reduce false "no speech" on quiet mics.
		return 35.0
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		return 35.0
	}
	return f
}

func loadRecordDuration() int {
	v := strings.TrimSpace(os.Getenv("MINI_TMK_RECORD_DURATION_SEC"))
	if v == "" {
		return 3
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 || n > 10 {
		return 3
	}
	return n
}

func shouldSilenceALSALogs() bool {
	v := strings.TrimSpace(os.Getenv("MINI_TMK_ALSA_SILENT"))
	if v == "" {
		return true
	}
	return v != "0" && strings.ToLower(v) != "false"
}

func withStderrSilenced(fn func() error) error {
	stderrFD := int(os.Stderr.Fd())
	backupFD, err := syscall.Dup(stderrFD)
	if err != nil {
		return fn()
	}
	defer syscall.Close(backupFD)

	nullFile, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return fn()
	}
	defer nullFile.Close()

	if err := syscall.Dup2(int(nullFile.Fd()), stderrFD); err != nil {
		return fn()
	}
	defer syscall.Dup2(backupFD, stderrFD)

	return fn()
}

func HasSpeech(samples []int16, threshold float64) bool {
	var sum float64

	for _, s := range samples {
		sum += math.Abs(float64(s))
	}

	avg := sum / float64(len(samples))

	return avg > threshold
}

func RecordWav(filePath string) error {
	recordOnce := func() error {
		portaudio.Initialize()
		defer portaudio.Terminate()

		sampleRate := 16000
		buffer := make([]int16, sampleRate*recordDuration)

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
		if err := stream.Stop(); err != nil {
			return err
		}

		if !HasSpeech(buffer, threshold) {
			return fmt.Errorf("no speech detected")
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

	if shouldSilenceALSALogs() {
		return withStderrSilenced(recordOnce)
	}

	return recordOnce()
}
