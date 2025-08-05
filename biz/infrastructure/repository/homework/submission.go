package homework

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type HomeworkSubmission struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	HomeworkID  string             `bson:"homework_id" json:"homeworkId"`
	StudentName string             `bson:"student_name" json:"studentName"`
	Title       string             `bson:"title" json:"title"`
	Text        string             `bson:"text" json:"text"`
	Images      []string           `bson:"images" json:"images"`
	GradeResult string             `bson:"grade_result" json:"gradeResult"`
	Comment     string             `bson:"comment" json:"comment"`
	IsGraded    bool               `bson:"is_graded" json:"isGraded"`
	SubmitTime  time.Time          `bson:"submit_time" json:"submitTime"`
	CreateTime  time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime  time.Time          `bson:"update_time" json:"updateTime"`
}
