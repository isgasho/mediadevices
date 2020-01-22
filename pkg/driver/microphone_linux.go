package driver

import (
	"io"

	"github.com/jfreymuth/pulse"
	"github.com/pion/mediadevices/pkg/io/audio"
)

type microphone struct {
	c           *pulse.Client
	s           *pulse.RecordStream
	samplesChan chan<- []float32
}

var _ AudioAdapter = &microphone{}

func init() {
	GetManager().Register(&microphone{})
}

func (m *microphone) Open() error {
	var err error
	m.c, err = pulse.NewClient()
	if err != nil {
		return err
	}

	return nil
}

func (m *microphone) Close() error {
	m.c.Close()
	if m.s != nil {
		m.s.Close()
	}

	return nil
}

func (m *microphone) Start(setting AudioSetting) (audio.Reader, error) {
	var options []pulse.RecordOption
	if setting.Mono {
		options = append(options, pulse.RecordMono)
	} else {
		options = append(options, pulse.RecordStereo)
	}
	options = append(options, pulse.RecordSampleRate(48000), pulse.RecordBufferFragmentSize(512))

	samplesChan := make(chan []float32, 1)
	var buff []float32
	var bi int
	var more bool

	reader := audio.ReaderFunc(func(samples [][2]float32) (n int, err error) {
		for i := range samples {
			// if we don't have anything left in buff, we'll wait until we receive
			// more samples
			if bi == len(buff) {
				buff, more = <-samplesChan
				if !more {
					return i, io.EOF
				}
				bi = 0
			}

			samples[i][0] = buff[bi]
			if !setting.Mono {
				samples[i][1] = buff[bi+1]
				bi++
			}
			bi++
		}

		return len(samples), nil
	})

	handler := func(b []float32) {
		samplesChan <- b
	}

	stream, err := m.c.NewRecord(handler, options...)
	if err != nil {
		return nil, err
	}

	stream.Start()
	m.s = stream
	m.samplesChan = samplesChan
	return reader, nil
}

func (m *microphone) Stop() error {
	close(m.samplesChan)
	m.s.Stop()
	return nil
}

func (m *microphone) Info() Info {
	return Info{
		Kind:       Audio,
		DeviceType: Microphone,
	}
}

func (m *microphone) Settings() []AudioSetting {
	return []AudioSetting{AudioSetting{
		SampleRate: 48000,
		Mono:       false,
	}}
}