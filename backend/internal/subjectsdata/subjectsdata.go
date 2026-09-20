// Package subjectsdata holds static seed/validation data for subjects and
// grades — the seed list itself lives in schema.sql; VALID_GRADES here is
// used to validate incoming requests.
package subjectsdata

// ValidGrades mirrors the Python backend's VALID_GRADES (1-12).
func ValidGrade(grade int) bool {
	return grade >= 1 && grade <= 12
}
