package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"banksoal/internal/store"
)

var qtypeLabels = map[string]string{
	"pilihan_ganda": "pilihan ganda (4 opsi: A, B, C, D)",
	"benar_salah":   "benar/salah",
	"isian":         "isian singkat",
	"deskripsi":     "uraian/deskripsi (jawaban berupa paragraf)",
}

var validTypes = map[string]bool{
	"pilihan_ganda": true, "benar_salah": true, "isian": true, "deskripsi": true,
}

// SplitCounts mirrors _split_counts: ~40% pilihan_ganda, ~20% benar_salah,
// the rest isian.
func SplitCounts(total int) map[string]int {
	pg := roundHalfEven(float64(total) * 0.4)
	bs := roundHalfEven(float64(total) * 0.2)
	isian := total - pg - bs
	return map[string]int{"pilihan_ganda": pg, "benar_salah": bs, "isian": isian}
}

// roundHalfEven mirrors Python's round() (banker's rounding) closely enough
// for these small integer counts.
func roundHalfEven(v float64) int {
	floor := int(v)
	diff := v - float64(floor)
	switch {
	case diff < 0.5:
		return floor
	case diff > 0.5:
		return floor + 1
	default:
		if floor%2 == 0 {
			return floor
		}
		return floor + 1
	}
}

func buildPrompt(subject string, grade int, counts map[string]int, material string) string {
	var countsParts []string
	for _, qtype := range []string{"pilihan_ganda", "benar_salah", "isian", "deskripsi"} {
		count := counts[qtype]
		if count > 0 {
			countsParts = append(countsParts, fmt.Sprintf("%d soal %s", count, qtypeLabels[qtype]))
		}
	}
	countsText := strings.Join(countsParts, ", ")

	var materiText string
	if material != "" {
		trimmed := material
		if len(trimmed) > 4000 {
			trimmed = trimmed[:4000]
		}
		materiText = "Gunakan materi ajar berikut sebagai sumber utama soal:\n\n" + trimmed + "\n\n"
	} else {
		materiText = fmt.Sprintf("Tidak ada materi khusus. Gunakan kurikulum sekolah Indonesia umum untuk mata pelajaran %s kelas %d.\n\n", subject, grade)
	}

	return fmt.Sprintf(`Buat soal ujian mata pelajaran %s untuk kelas %d sekolah Indonesia. Komposisi soal: %s.

%sSemua teks soal, opsi, jawaban, dan pembahasan HARUS dalam Bahasa Indonesia yang sesuai untuk jenjang kelas tersebut. Jika materi memuat kutipan ayat Al-Qur'an, Hadis, atau istilah/frasa berbahasa Arab, PERTAHANKAN teks Arabnya persis apa adanya (jangan ditransliterasi ke huruf Latin, jangan hanya diterjemahkan tanpa teks aslinya); sertakan juga arti/terjemahannya dalam Bahasa Indonesia bila relevan.

Jika soal memuat rumus, persamaan, pecahan, pangkat, akar, atau notasi matematika lain, tulis menggunakan LaTeX: gunakan $...$ untuk notasi sebaris (contoh: $x^2 + 1$) dan $$...$$ untuk persamaan berdiri sendiri (contoh: $$\frac{a}{b} = c$$). Ini boleh muncul di pertanyaan, opsi, maupun pembahasan. Selain notasi matematika ini, jangan gunakan format markdown lain (tanpa bold, tanpa list, tanpa heading) — teks biasa saja.

Untuk SEBAGIAN KECIL soal saja (jangan berlebihan) di mana gambar benar-benar diperlukan agar soal bisa dipahami/dijawab (mis. diagram geometri, peta, grafik, atau mengenali objek/hewan/tumbuhan/tempat nyata), tambahkan dua field berikut pada objek soal itu:
- "gambar_tipe": "generated" jika gambar berupa ilustrasi/diagram yang perlu dibuat (sertakan juga "gambar_prompt": deskripsi singkat gambar yang harus dibuat), atau "gambar_tipe": "stock" jika yang dibutuhkan adalah foto benda/tempat/makhluk nyata (sertakan juga "gambar_cari": kata kunci pencarian foto singkat dalam Bahasa Inggris).
Soal lain yang tidak butuh gambar TIDAK PERLU menyertakan field ini sama sekali.

Balas HANYA dengan JSON valid (tanpa teks lain) dengan format:
{"questions": [
  {"tipe": "pilihan_ganda", "pertanyaan": "...", "opsi": ["...", "...", "...", "..."], "jawaban": 0, "pembahasan": "..."},
  {"tipe": "benar_salah", "pertanyaan": "...", "jawaban": "benar" atau "salah", "pembahasan": "..."},
  {"tipe": "isian", "pertanyaan": "...", "jawaban": "jawaban singkat", "pembahasan": "...", "gambar_tipe": "stock", "gambar_cari": "..."}
  {"tipe": "deskripsi", "pertanyaan": "...", "jawaban": "uraian jawaban model berupa beberapa kalimat", "pembahasan": "..."}
]}

Untuk pilihan_ganda, jawaban adalah indeks opsi yang benar (0-3). Untuk deskripsi, jawaban adalah jawaban model berupa uraian lengkap (beberapa kalimat) yang memuat seluruh poin penting yang diharapkan dari siswa. Pembahasan harus menjelaskan mengapa jawaban tersebut benar.`, subject, grade, countsText, materiText)
}

func hasBalancedMathDelimiters(text string) bool {
	count := 0
	escaped := false
	for _, ch := range text {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
		} else if ch == '$' {
			count++
		}
	}
	return count%2 == 0
}

// RawQuestion is one question item as returned by the LLM, before image
// resolution turns gambar_tipe/gambar_prompt/gambar_cari into a real
// "gambar" URL (see ResolvedQuestion / the quiz handler's image step).
type RawQuestion struct {
	Tipe         string          `json:"tipe"`
	Pertanyaan   string          `json:"pertanyaan"`
	Opsi         []string        `json:"opsi,omitempty"`
	Jawaban      json.RawMessage `json:"jawaban"`
	Pembahasan   string          `json:"pembahasan"`
	GambarTipe   string          `json:"gambar_tipe,omitempty"`
	GambarPrompt string          `json:"gambar_prompt,omitempty"`
	GambarCari   string          `json:"gambar_cari,omitempty"`
	Gambar       string          `json:"-"`
}

// ToStoreQuestion converts a validated RawQuestion (after image resolution)
// into the store.Question shape persisted in the quizzes.questions column.
func (q RawQuestion) ToStoreQuestion() (store.Question, error) {
	out := store.Question{
		Tipe:       q.Tipe,
		Pertanyaan: q.Pertanyaan,
		Opsi:       q.Opsi,
		Pembahasan: q.Pembahasan,
		Gambar:     q.Gambar,
	}
	if q.Tipe == "pilihan_ganda" {
		var idx int
		if err := json.Unmarshal(q.Jawaban, &idx); err != nil {
			return out, err
		}
		out.Jawaban = idx
	} else {
		var s string
		if err := json.Unmarshal(q.Jawaban, &s); err != nil {
			return out, err
		}
		out.Jawaban = s
	}
	return out, nil
}

type questionsPayload struct {
	Questions []RawQuestion `json:"questions"`
}

func validateQuestions(raw []byte, counts map[string]int) ([]RawQuestion, error) {
	var payload questionsPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("JSON tidak berisi daftar questions")
	}
	questions := payload.Questions

	expected := map[string]int{}
	total := 0
	for t, c := range counts {
		if c > 0 {
			expected[t] = c
			total += c
		}
	}
	actual := map[string]int{}
	for _, q := range questions {
		if q.Tipe != "" {
			actual[q.Tipe]++
		}
	}
	if len(questions) != total || !intMapsEqual(actual, expected) {
		return nil, fmt.Errorf("jumlah/komposisi soal tidak sesuai: dapat %v soal, seharusnya %v (total %d)", actual, expected, total)
	}

	for i, q := range questions {
		if !validTypes[q.Tipe] {
			return nil, fmt.Errorf("soal %d: tipe tidak valid: %s", i, q.Tipe)
		}
		if strings.TrimSpace(q.Pertanyaan) == "" || strings.TrimSpace(q.Pembahasan) == "" {
			return nil, fmt.Errorf("soal %d: pertanyaan/pembahasan kosong", i)
		}
		textFields := []string{q.Pertanyaan, q.Pembahasan}

		switch q.Tipe {
		case "pilihan_ganda":
			if len(q.Opsi) != 4 {
				return nil, fmt.Errorf("soal %d: opsi harus 4 item", i)
			}
			var idx int
			if err := json.Unmarshal(q.Jawaban, &idx); err != nil {
				return nil, fmt.Errorf("soal %d: jawaban harus indeks 0-3", i)
			}
			if idx < 0 || idx > 3 {
				return nil, fmt.Errorf("soal %d: jawaban harus indeks 0-3", i)
			}
			textFields = append(textFields, q.Opsi...)
		case "benar_salah":
			var s string
			_ = json.Unmarshal(q.Jawaban, &s)
			if s != "benar" && s != "salah" {
				return nil, fmt.Errorf("soal %d: jawaban harus 'benar' atau 'salah'", i)
			}
		case "isian", "deskripsi":
			var s string
			_ = json.Unmarshal(q.Jawaban, &s)
			if strings.TrimSpace(s) == "" {
				return nil, fmt.Errorf("soal %d: jawaban %s kosong", i, q.Tipe)
			}
		}

		for _, field := range textFields {
			if !hasBalancedMathDelimiters(field) {
				return nil, fmt.Errorf("soal %d: delimiter $ tidak seimbang", i)
			}
		}

		if q.GambarTipe != "" {
			if q.GambarTipe != "generated" && q.GambarTipe != "stock" {
				return nil, fmt.Errorf("soal %d: gambar_tipe tidak valid: %s", i, q.GambarTipe)
			}
			if q.GambarTipe == "generated" && strings.TrimSpace(q.GambarPrompt) == "" {
				return nil, fmt.Errorf("soal %d: gambar_prompt kosong", i)
			}
			if q.GambarTipe == "stock" && strings.TrimSpace(q.GambarCari) == "" {
				return nil, fmt.Errorf("soal %d: gambar_cari kosong", i)
			}
		}
	}
	return questions, nil
}

func intMapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// GenerateQuiz asks the LLM for one quiz package's worth of questions,
// retrying once with a JSON-repair follow-up message if validation fails.
func (c *Client) GenerateQuiz(ctx context.Context, subject string, grade int, counts map[string]int, material string) ([]RawQuestion, error) {
	prompt := buildPrompt(subject, grade, counts, material)
	total := 0
	for _, v := range counts {
		if v > 0 {
			total += v
		}
	}
	messages := []chatMessage{
		{Role: "system", Content: "Anda pembuat soal ujian sekolah Indonesia. Anda selalu menjawab dengan JSON valid saja."},
		{Role: "user", Content: prompt},
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		content, err := c.chat(ctx, messages)
		if err != nil {
			return nil, err
		}
		questions, verr := validateQuestions([]byte(content), counts)
		if verr == nil {
			return questions, nil
		}
		lastErr = verr
		messages = append(messages[:2], chatMessage{Role: "assistant", Content: content}, chatMessage{
			Role:    "user",
			Content: fmt.Sprintf("JSON Anda tidak valid: %s. Perbaiki dan balas HANYA JSON valid sesuai format, tepat %d soal.", verr, total),
		})
	}
	return nil, newError("Gagal membuat soal setelah beberapa percobaan. Coba lagi beberapa saat lagi. (%v)", lastErr)
}
