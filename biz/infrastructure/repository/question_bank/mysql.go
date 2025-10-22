package question_bank

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"essay-show/biz/application/dto/essay/show"
	"essay-show/biz/infrastructure/util/log"

	_ "github.com/go-sql-driver/mysql"
)

type MySQLMapper struct {
	db *sql.DB
}

// Essay 对应数据库中的 Essays 表
type Essay struct {
	ID              int     `db:"id"`
	Type            int     `db:"type"`
	TextbookVersion *int    `db:"textbook_version"`
	Grade           *int    `db:"grade"`
	Unit            *int    `db:"unit"`
	Name            *string `db:"name"`
	Description     *string `db:"description"`
	Genre           *string `db:"genre"`
}

func NewMySQLMapper(dsn string) (*MySQLMapper, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql connection: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping mysql: %w", err)
	}

	log.Info("MySQL connection established successfully")
	return &MySQLMapper{db: db}, nil
}

func (m *MySQLMapper) Close() error {
	return m.db.Close()
}

// ListQuestionBanks 获取题库列表
func (m *MySQLMapper) ListQuestionBanks(ctx context.Context, req *show.ListQuestionBanksReq) ([]*show.QuestionBank, int64, error) {
	// 构建查询条件
	var conditions []string
	var args []interface{}

	// 按类型筛选 (0-课内题库, 1-写作训练, 2-课外题库)
	if req.Type == show.QuestionBankType_IN_CLASS {
		conditions = append(conditions, "type = ?")
		args = append(args, 0) // 课内题库
	}

	// 按年级筛选
	if len(req.Grade) > 0 {
		placeholders := make([]string, len(req.Grade))
		for i, grade := range req.Grade {
			placeholders[i] = "?"
			args = append(args, grade)
		}
		conditions = append(conditions, fmt.Sprintf("grade IN (%s)", strings.Join(placeholders, ",")))
	}

	// 构建 WHERE 子句
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 获取总数
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM Essays %s", whereClause)
	var total int64
	err := m.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		log.Error("Failed to count question banks: %v", err)
		return nil, 0, fmt.Errorf("failed to count question banks: %w", err)
	}

	// 分页参数
	page := int64(1)
	limit := int64(10)
	if req.PaginationOptions != nil {
		if req.PaginationOptions.Page != nil {
			page = *req.PaginationOptions.Page
		}
		if req.PaginationOptions.Limit != nil {
			limit = *req.PaginationOptions.Limit
		}
	}

	offset := (page - 1) * limit

	// 查询数据
	dataQuery := fmt.Sprintf(`
		SELECT id, type, textbook_version, grade, unit, name, description, genre 
		FROM Essays %s 
		ORDER BY grade ASC, unit ASC, id ASC 
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, offset)

	rows, err := m.db.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		log.Error("Failed to query question banks: %v", err)
		return nil, 0, fmt.Errorf("failed to query question banks: %w", err)
	}
	defer rows.Close()

	var questionBanks []*show.QuestionBank
	for rows.Next() {
		var essay Essay
		err := rows.Scan(
			&essay.ID,
			&essay.Type,
			&essay.TextbookVersion,
			&essay.Grade,
			&essay.Unit,
			&essay.Name,
			&essay.Description,
			&essay.Genre,
		)
		if err != nil {
			log.Error("Failed to scan essay row: %v", err)
			continue
		}

		// 转换为 QuestionBank 结构
		questionBank := &show.QuestionBank{
			Id:          strconv.Itoa(essay.ID),
			Name:        safeString(essay.Name),
			Description: safeString(essay.Description),
			Grade:       safeInt64(essay.Grade),
			Unit:        safeInt64(essay.Unit),
			EssayType:   safeString(essay.Genre),
		}

		questionBanks = append(questionBanks, questionBank)
	}

	if err = rows.Err(); err != nil {
		log.Error("Error iterating over rows: %v", err)
		return nil, 0, fmt.Errorf("error iterating over rows: %w", err)
	}

	return questionBanks, total, nil
}

// safeString 安全地将 *string 转换为 string
func safeString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// safeInt64 安全地将 *int 转换为 int64
func safeInt64(i *int) int64 {
	if i == nil {
		return 0
	}
	return int64(*i)
}
