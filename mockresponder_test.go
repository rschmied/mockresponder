package mockresponder

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockResponder_SetDoFunc(t *testing.T) {
	mrClient, _ := NewMockResponder()
	dummyDoFunc := func(req *http.Request) (*http.Response, error) {
		return &http.Response{}, nil
	}

	fn1 := runtime.FuncForPC(reflect.ValueOf(mrClient.doFunc).Pointer()).Name()
	fn2 := runtime.FuncForPC(reflect.ValueOf(defaultDoFunc).Pointer()).Name()
	assert.Equal(t, fn1, fn2)

	mrClient.SetDoFunc(dummyDoFunc)

	fn1 = runtime.FuncForPC(reflect.ValueOf(mrClient.doFunc).Pointer()).Name()
	fn2 = runtime.FuncForPC(reflect.ValueOf(dummyDoFunc).Pointer()).Name()
	assert.Equal(t, fn1, fn2)
}

func TestMockResponder_SetData(t *testing.T) {
	mrClient, _ := NewMockResponder()
	data := MockRespList{
		MockResp{served: true},
		MockResp{served: true},
		MockResp{},
		MockResp{},
	}
	mrClient.SetData(data)
	assert.Equal(t, len(mrClient.mockData), 4)
}

func TestMockResponder_GetData(t *testing.T) {
	mrClient, _ := NewMockResponder()
	data := MockRespList{
		MockResp{Code: 200},
		MockResp{Code: 204},
		MockResp{Code: 401},
		MockResp{Code: 404},
	}
	mrClient.SetData(data)
	newData := mrClient.GetData()
	assert.Equal(t, data, newData)
}

func TestMockResponder_Empty(t *testing.T) {
	mrClient, _ := NewMockResponder()
	data := MockRespList{
		MockResp{served: true},
		MockResp{served: true},
		MockResp{served: true},
	}
	mrClient.mockData = data
	assert.True(t, mrClient.Empty())
	mrClient.Reset()
	assert.False(t, mrClient.Empty())
	assert.Equal(t, mrClient.lastServed, 0)
	for _, mr := range mrClient.mockData {
		assert.False(t, mr.served)
	}
}

func TestMockResponder_LastData(t *testing.T) {
	mrClient, _ := NewMockResponder()
	data := MockRespList{
		MockResp{Data: []byte(`OK`), served: true},
		MockResp{Data: []byte(`NAK`), served: false},
		MockResp{Data: []byte(`BLA`), served: false},
	}
	mrClient.mockData = data
	mrClient.lastServed = 1
	lastData := mrClient.LastData()
	assert.Equal(t, lastData, data[1].Data)
}

func TestMockResponder_Do(t *testing.T) {
	mrClient, ctx := NewMockResponder()
	data := MockRespList{
		MockResp{Data: []byte(`NAK`), URL: "auth$", Err: errors.New("ugh")},
		MockResp{Data: []byte(`OK`), URL: "ok$"},
		MockResp{Data: []byte(`BLA`)},
	}
	mrClient.SetData(data)

	tests := []struct {
		name    string
		url     string
		want    []byte
		wantErr bool
	}{
		{"good", "bla://bla/ok", []byte(`OK`), false},
		{"witherr", "bla://bla/auth", nil, true},
		{"last", "bla://bla/auth", []byte(`BLA`), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, tt.url, nil)
			resp, err := mrClient.Do(req)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, body)
		})
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "notfound", nil)
	pf := func() {
		mrClient.Do(req)
	}
	assert.Panics(t, pf)

}

func TestMockResponder_PanicsInDo(t *testing.T) {
	mrClient, ctx := NewMockResponder()
	data := MockRespList{
		MockResp{URL: "* * *"},
	}
	mrClient.SetData(data)

	req, _ := http.NewRequestWithContext(context.TODO(), http.MethodGet, "", nil)
	pf1 := func() {
		mrClient.Do(req)
	}
	assert.Panics(t, pf1)

	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, "", nil)
	pf2 := func() {
		mrClient.Do(req)
	}
	assert.Panics(t, pf2)

	bogusCtx := context.WithValue(context.TODO(), contextMockClient, data)
	req, _ = http.NewRequestWithContext(bogusCtx, http.MethodGet, "", nil)
	pf3 := func() {
		mrClient.Do(req)
	}
	assert.Panics(t, pf3)

	var (
		mri interface{}
		p   *MockResponder = nil
	)
	mri = p

	bogusCtx = context.WithValue(context.TODO(), contextMockClient, mri)
	req, _ = http.NewRequestWithContext(bogusCtx, http.MethodGet, "", nil)
	pf4 := func() {
		mrClient.Do(req)
	}
	assert.Panics(t, pf4)

}
