package homework

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type HomeworkSubmission struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	HomeworkID  string             `bson:"homework_id" json:"homeworkId"`
	StudentID   string             `bson:"student_id" json:"studentId"`
	TeacherID   string             `bson:"teacher_id" json:"teacherId"`
	Images      []string           `bson:"images" json:"images"`
	GradeResult string             `bson:"grade_result" json:"gradeResult"`
	Title       string             `bson:"title" json:"title"`
	Response    string             `bson:"response" json:"response"`
	Message     string             `bson:"message" json:"message"`
	Status      int                `bson:"status" json:"status"` // 0: 初始化, 1: 批改中, 2: 批改完成, 3: 批改已人工修改, 7:批改失败
	CreateTime  time.Time          `bson:"create_time" json:"createTime"`
	UpdateTime  time.Time          `bson:"update_time" json:"updateTime"`
}
