package util

func GetGradeType(grade int64) string {
	if grade < 7 {
		return "mid"
	}
	return "high"
}
