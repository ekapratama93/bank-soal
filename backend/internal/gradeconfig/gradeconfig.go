// Package gradeconfig holds the per-grade defaults used when an exam type
// doesn't specify its own question count / duration.
package gradeconfig

type Config struct {
	JumlahSoal  int
	DurasiMenit int
}

var byGrade = map[int]Config{
	1: {JumlahSoal: 20, DurasiMenit: 60},
	2: {JumlahSoal: 20, DurasiMenit: 60},
	3: {JumlahSoal: 25, DurasiMenit: 60},
	4: {JumlahSoal: 25, DurasiMenit: 60},
	5: {JumlahSoal: 30, DurasiMenit: 60},
	6: {JumlahSoal: 30, DurasiMenit: 60},
}

var defaultConfig = Config{JumlahSoal: 30, DurasiMenit: 90}

func Get(grade int) Config {
	if c, ok := byGrade[grade]; ok {
		return c
	}
	return defaultConfig
}
