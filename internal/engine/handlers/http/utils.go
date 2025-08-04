package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func reqToCtx(r *http.Request) context.Context {
	ctx := r.Context()

	for k, v := range r.Header {
		ctx = context.WithValue(ctx, strings.ToLower(k), v[0])
	}

	return ctx
}

func wrtRsp(w http.ResponseWriter, code int, body any) {
	bs, _ := json.Marshal(body)
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(code)
	fmt.Fprint(w, string(bs))
}
