package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"

	// "github.com/go-audio/aiff"
	// "github.com/go-audio/audio"
	"github.com/go-audio/audio"
	"github.com/gordonklaus/portaudio"
)

const (
	// silence detection cutoff
	cutoff     = math.MaxInt32 / 100
	sampleRate = 32000
)

func main() {
	portaudio.Initialize()
	defer portaudio.Terminate()
	d, err := portaudio.Devices()
	chk(err)
	for _, device := range d {
		fmt.Printf("device: %#v\n", device)
	}

	if len(os.Args) < 2 {
		fmt.Println("missing required argument: output file name")
		return
	}
	fmt.Println("Recording. Press Ctrl-C to stop.")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Kill)

	fileName := os.Args[1]
	if !strings.HasSuffix(fileName, ".aiff") {
		fileName += ".aiff"
	}
	f, err := os.Create(fileName)
	chk(err)

	// form chunk
	_, err = f.WriteString("FORM")
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(0))) //total bytes
	_, err = f.WriteString("AIFF")
	chk(err)

	// common chunk
	_, err = f.WriteString("COMM")
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(18))) //size
	chk(binary.Write(f, binary.BigEndian, int16(1)))  //channels
	chk(binary.Write(f, binary.BigEndian, int32(0)))  //number of samples
	chk(binary.Write(f, binary.BigEndian, int16(32))) //bits per sample
	s := audio.IntToIEEEFloat(int(sampleRate))
	_, err = f.Write(s[:])
	chk(err)

	// sound chunk
	_, err = f.WriteString("SSND")
	chk(err)
	chk(binary.Write(f, binary.BigEndian, int32(0))) //size
	chk(binary.Write(f, binary.BigEndian, int32(0))) //offset
	chk(binary.Write(f, binary.BigEndian, int32(0))) //block
	nSamples := 0
	defer func() {
		// fill in missing sizes
		totalBytes := 4 + 8 + 18 + 8 + 8 + 4*nSamples
		_, err = f.Seek(4, 0)
		chk(err)
		chk(binary.Write(f, binary.BigEndian, int32(totalBytes)))
		_, err = f.Seek(22, 0)
		chk(err)
		chk(binary.Write(f, binary.BigEndian, int32(nSamples)))
		_, err = f.Seek(42, 0)
		chk(err)
		chk(binary.Write(f, binary.BigEndian, int32(4*nSamples+8)))
		chk(f.Close())
	}()

	portaudio.Initialize()
	defer portaudio.Terminate()
	in := make([]int32, 64)
	stream, err := portaudio.OpenDefaultStream(1, 0, float64(sampleRate), len(in), in)
	chk(err)
	defer stream.Close()

	chk(stream.Start())
	silence := 0
	sound := 0
	inSilence := false
	for {
		chk(stream.Read())
		nSamples += len(in)
		// fmt.Printf("%d (% 8d): ", len(in), nSamples)
		for _, v := range in {
			if (-cutoff < v && v < 0) || (0 < v && v < cutoff) {
				// fmt.Printf(" ")
				silence++
				sound = 0
			} else {
				silence = 0
				sound++
				if inSilence {
					voicedetected()
				}
				inSilence = false
				// fmt.Printf("*")
			}
		}
		// 3 seconds of silence
		if silence >= sampleRate*3 {
			// consider this silence!
			if !inSilence {
				inSilence = true
				fmt.Println("==================== SILENCE ====================")
			}
			s := make([]int32, 64)
			chk(binary.Write(f, binary.BigEndian, s))
		} else {
			chk(binary.Write(f, binary.BigEndian, in))
		}
		select {
		case <-sig:
			return
		default:
		}
	}
	chk(stream.Stop())
}

func chk(err error) {
	if err != nil {
		panic(err)
	}
}

func voicedetected() {
	fmt.Println("======================= AUDIO ========================")
}
