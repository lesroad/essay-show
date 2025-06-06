package adaptor

import (
	"context"
	"essay-show/biz/infrastructure/consts"
	"essay-show/biz/infrastructure/util"
	"essay-show/biz/infrastructure/util/log"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	hertz "github.com/cloudwego/hertz/pkg/protocol/consts"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc/status"
)

var _ propagation.TextMapCarrier = &headerProvider{}

type headerProvider struct {
	headers *protocol.ResponseHeader
}

// Get a value from metadata by key
func (m *headerProvider) Get(key string) string {
	return m.headers.Get(key)
}

// Set a value to metadata by k/v
func (m *headerProvider) Set(key, value string) {
	m.headers.Set(key, value)
}

// Keys Iteratively get all keys of metadata
func (m *headerProvider) Keys() []string {
	out := make([]string, 0)

	m.headers.VisitAll(func(key, value []byte) {
		out = append(out, string(key))
	})

	return out
}

func PostProcess(ctx context.Context, c *app.RequestContext, req, resp any, err error) {
	log.CtxInfo(ctx, "[%s] req=%s, resp=%s, err=%v", c.Path(), util.JSONF(req), util.JSONF(resp), err)
	b3.New().Inject(ctx, &headerProvider{headers: &c.Response.Header})

	switch err {
	case nil:
		c.JSON(hertz.StatusOK, resp)
	case consts.ErrForbidden:
		c.JSON(hertz.StatusForbidden, err.Error())
	default:
		if s, ok := status.FromError(err); ok {
			c.JSON(http.StatusOK, &BizError{
				Code: uint32(s.Code()),
				Msg:  s.Message(),
			})
		} else {
			log.CtxError(ctx, "internal error, err=%s", err.Error())
			code := hertz.StatusInternalServerError
			c.String(code, hertz.StatusMessage(code))
		}
	}
}

type BizError struct {
	Code uint32 `json:"code"`
	Msg  string `json:"msg"`
}
