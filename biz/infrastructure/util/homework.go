package util

func GetGradeType(grade int64) string {
	if grade < 10 {
		return "mid"
	} else if grade < 13 {
		return "high"
	} else {
		return "hsk"
	}
}
