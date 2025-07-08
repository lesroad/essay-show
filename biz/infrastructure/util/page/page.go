package util

import "essay-show/biz/application/dto/basic"

func ParsePageOpt(p *basic.PaginationOptions) (skip int64, limit int64) {
	// 设置分页参数
	skip = int64(0)
	limit = int64(5) // 默认限制为10条数据

	if p != nil && p.Page != nil && p.Limit != nil {
		skip = (*p.Page - 1) * *p.Limit
		limit = *p.Limit
	}
	return skip, limit
}
