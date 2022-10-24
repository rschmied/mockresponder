package mockresponder

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

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

	// context has no contextMockClient context key
	req, _ := http.NewRequestWithContext(context.TODO(), http.MethodGet, "", nil)
	panicFunc := func() {
		mrClient.Do(req)
	}
	assert.Panics(t, panicFunc)

	// this panics because of the invalid regex set above
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, "", nil)
	assert.Panics(t, panicFunc)

	// this has the correct context key but the value is not a MockResponder
	bogusCtx := context.WithValue(context.TODO(), contextMockClient, data)
	req, _ = http.NewRequestWithContext(bogusCtx, http.MethodGet, "", nil)
	assert.Panics(t, panicFunc)

	var (
		mri any
		p   *MockResponder = nil
	)
	mri = p

	// this has a nil MockResponder / interface
	bogusCtx = context.WithValue(context.TODO(), contextMockClient, mri)
	req, _ = http.NewRequestWithContext(bogusCtx, http.MethodGet, "", nil)
	assert.Panics(t, panicFunc)

}

func Test_sanitizeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"ok", "ok test", "ok test"},
		{"nl", "ok\ntest", "oktest"},
		{"cr", "ok\rtest", "oktest"},
		{"nlcr", "ok\rtest\nbla", "oktestbla"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeURL(tt.url); got != tt.want {
				t.Errorf("sanitizeURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_Race(t *testing.T) {

	mrClient, ctx := NewMockResponder()
	data := MockRespList{
		MockResp{Code: 200},
		MockResp{Code: 200},
	}
	mrClient.SetData(data)

	done := false
	wg := sync.WaitGroup{}
	wg.Add(2)

	mu := sync.Mutex{}

	get := func() {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/bla", nil)
		resp, _ := mrClient.Do(req)
		defer resp.Body.Close()
		io.ReadAll(resp.Body)
	}

	go func() {
		get()
		wg.Done()
	}()

	go func() {
		get()
		wg.Done()
	}()

	go func() {
		wg.Wait()
		mu.Lock()
		done = true
		mu.Unlock()
	}()

	doneCheck := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return done
	}

	assert.Eventually(t, doneCheck, time.Second*5, time.Microsecond*50)
	assert.True(t, mrClient.Empty())
}
