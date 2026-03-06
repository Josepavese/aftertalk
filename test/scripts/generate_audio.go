package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"time"
)

// ConversationScript represents a spoken dialogue
type ConversationScript struct {
	Speaker  string
	Text     string
	Duration time.Duration
}

// Client conversation (worried client)
var clientScript = []ConversationScript{
	{"Client", "Buongiorno, sono Maria Rossi. Ho urgente bisogno di aiuto.", 4 * time.Second},
	{"Client", "Il mio problema riguarda il mio account che non riesco ad accedere da tre giorni.", 6 * time.Second},
	{"Client", "Ho provato a reimpostare la password ma non ricevo l'email.", 5 * time.Second},
	{"Client", "Questo è molto importante perché devo accedere ai documenti per domani.", 5 * time.Second},
	{"Client", "Sì, ho controllato anche nello spam ma non c'è nulla.", 4 * time.Second},
	{"Client", "Il mio indirizzo email è maria.rossi@email.com.", 4 * time.Second},
	{"Client", "Ah, capisco! Proverò subito. Grazie mille per l'aiuto!", 5 * time.Second},
}

// Professional conversation (helpful operator)
var professionalScript = []ConversationScript{
	{"Professional", "Buongiorno Maria, sono l'operatore Marco. Come posso aiutarla oggi?", 5 * time.Second},
	{"Professional", "Capisco la sua frustrazione. Verifichiamo subito cosa sta succedendo.", 5 * time.Second},
	{"Professional", "Posso chiederle se ha controllato la cartella dello spam?", 4 * time.Second},
	{"Professional", "Ho capito. Controllo immediatamente il suo account nel sistema.", 4 * time.Second},
	{"Professional", "Ho trovato il problema! L'email era bloccata. La sblocco ora.", 5 * time.Second},
	{"Professional", "Perfetto, ho sbloccato l'indirizzo. Ora dovrebbe ricevere l'email entro due minuti.", 6 * time.Second},
	{"Professional", "Prego Maria! Se ha altri problemi non esiti a contattarci. Buona giornata!", 6 * time.Second},
}

// generateVoiceLikeAudio creates synthetic PCM audio that mimics speech patterns
func generateVoiceLikeAudio(script []ConversationScript, outputPath string) error {
	const (
		sampleRate = 16000
		channels   = 1
		bits       = 16
	)

	totalDuration := time.Duration(0)
	for _, line := range script {
		totalDuration += line.Duration
	}

	totalSamples := int(float64(sampleRate) * totalDuration.Seconds())
	samples := make([]int16, totalSamples)

	sampleIndex := 0
	for _, line := range script {
		lineSamples := int(float64(sampleRate) * line.Duration.Seconds())

		// Generate voice-like audio using multiple sine waves (formants)
		for i := 0; i < lineSamples; i++ {
			t := float64(i) / float64(sampleRate)

			// Base frequency (fundamental - voice pitch ~120-250Hz)
			baseFreq := 150.0 + math.Sin(t*2)*30 // Varying pitch

			// Formants (harmonics that create vowel sounds)
			f1 := baseFreq * 1.0 // First formant
			f2 := baseFreq * 2.5 // Second formant
			f3 := baseFreq * 3.2 // Third formant
			f4 := baseFreq * 4.1 // Fourth formant

			// Amplitude envelope (simulates syllables)
			syllableRate := 4.0 // syllables per second
			envelope := 0.5 + 0.5*math.Sin(t*syllableRate*2*math.Pi)

			// Combine frequencies with different weights
			sample := 0.0
			sample += math.Sin(2*math.Pi*f1*t) * 0.4
			sample += math.Sin(2*math.Pi*f2*t) * 0.3
			sample += math.Sin(2*math.Pi*f3*t) * 0.2
			sample += math.Sin(2*math.Pi*f4*t) * 0.1

			// Add some noise for realism
			noise := (mathrand() - 0.5) * 0.05
			sample += noise

			// Apply envelope
			sample *= envelope

			// Scale to 16-bit range
			amplitude := int16(sample * 8000)

			if sampleIndex < len(samples) {
				samples[sampleIndex] = amplitude
				sampleIndex++
			}
		}
	}

	// Write as raw PCM
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write WAV header
	if err := writeWAVHeader(file, sampleRate, channels, bits, len(samples)); err != nil {
		return fmt.Errorf("failed to write WAV header: %w", err)
	}

	// Write samples
	for _, sample := range samples {
		if err := binary.Write(file, binary.LittleEndian, sample); err != nil {
			return fmt.Errorf("failed to write sample: %w", err)
		}
	}

	return nil
}

func writeWAVHeader(file *os.File, sampleRate, channels, bits, numSamples int) error {
	byteRate := sampleRate * channels * bits / 8
	blockAlign := channels * bits / 8
	dataSize := numSamples * channels * bits / 8

	// RIFF header
	file.Write([]byte("RIFF"))
	binary.Write(file, binary.LittleEndian, uint32(36+dataSize))
	file.Write([]byte("WAVE"))

	// fmt chunk
	file.Write([]byte("fmt "))
	binary.Write(file, binary.LittleEndian, uint32(16)) // Subchunk1Size
	binary.Write(file, binary.LittleEndian, uint16(1))  // AudioFormat (PCM)
	binary.Write(file, binary.LittleEndian, uint16(channels))
	binary.Write(file, binary.LittleEndian, uint32(sampleRate))
	binary.Write(file, binary.LittleEndian, uint32(byteRate))
	binary.Write(file, binary.LittleEndian, uint16(blockAlign))
	binary.Write(file, binary.LittleEndian, uint16(bits))

	// data chunk
	file.Write([]byte("data"))
	binary.Write(file, binary.LittleEndian, uint32(dataSize))

	return nil
}

// Simple pseudo-random for deterministic output
var randState uint32 = 12345

func mathrand() float64 {
	randState = (randState * 1103515245) + 12345
	return float64(randState) / float64(^uint32(0))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: generate_audio <output_dir>")
		os.Exit(1)
	}

	outputDir := os.Args[1]

	fmt.Println("🎵 Generazione audio di test realistici...")

	fmt.Println("  Generazione audio Client...")
	if err := generateVoiceLikeAudio(clientScript, outputDir+"/client_conversation.wav"); err != nil {
		fmt.Printf("  ❌ Errore: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✅ Audio Client generato")

	fmt.Println("  Generazione audio Professional...")
	if err := generateVoiceLikeAudio(professionalScript, outputDir+"/professional_conversation.wav"); err != nil {
		fmt.Printf("  ❌ Errore: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✅ Audio Professional generato")

	fmt.Println("")
	fmt.Println("✅ Audio di test generati con successo!")
	fmt.Printf("   📁 %s/client_conversation.wav\n", outputDir)
	fmt.Printf("   📁 %s/professional_conversation.wav\n", outputDir)
	fmt.Println("")
	fmt.Println("📝 Trascrizioni attese salvate in:")
	fmt.Printf("   📁 %s/client_transcription.txt\n", outputDir)
	fmt.Printf("   📁 %s/professional_transcription.txt\n", outputDir)

	// Save transcriptions for validation
	clientFile, _ := os.Create(outputDir + "/client_transcription.txt")
	for _, line := range clientScript {
		clientFile.WriteString(line.Text + "\n")
	}
	clientFile.Close()

	proFile, _ := os.Create(outputDir + "/professional_transcription.txt")
	for _, line := range professionalScript {
		proFile.WriteString(line.Text + "\n")
	}
	proFile.Close()
}
