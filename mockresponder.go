package mockresponder

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

// MockResp is a mock response, the URL can be a RegEx, in this case the first
// response in the list of unserved responses which matches the RegEx will be
// served.  If no Regex is provided, the first unserved response is served.  The
// default status code is 200, can be overwritten in Code.  If Err is provided,
// then this error will be returned.
type MockResp struct {
	Data   []byte
	Code   int
	URL    string
	Err    error
	served bool
}

func (mr MockResp) String() string {
	// return fmt.Sprintf("%s/%d/%v/%s", mr.URL, mr.Code, mr.Err, string(mr.Data))
	return fmt.Sprintf("%s/%d/%v/%v", mr.URL, mr.Code, mr.Err, mr.served)
}

// MockRespList is a list of mocked responses, these are the responses that the
// MockResponder serves, either sequentially or because of a RegEx match.
type MockRespList []MockResp

type contextKey string

const (
	contextMockClient = contextKey("mockclient")
)

// MockResponder serves mock responses
type MockResponder struct {
	doFunc     func(req *http.Request) (*http.Response, error)
	mockData   MockRespList
	lastServed int
	mu         sync.Mutex
}

func sanitizeURL(url string) string {
	return strings.Map(
		func(r rune) rune {
			switch r {
			case '\n':
				fallthrough
			case '\r':
				return -1
			}
			return r
		}, url)
}

// defaultDoFunc is the default implementation to return mocked responses
// as defined in the response list of the mock responder.
func defaultDoFunc(req *http.Request) (*http.Response, error) {
	ctxValue := req.Context().Value(contextMockClient)
	if ctxValue == nil {
		panic("no MockResponse context")
	}
	mc, ok := ctxValue.(*MockResponder)
	if !ok {
		panic("returned value is not a MockResponder!")
	}

	log.Printf("mock request url %s %s", req.Method, sanitizeURL(req.URL.String()))
	if mc == nil {
		panic("no data")
	}

	var (
		idx  int
		data MockResp
	)

	found := false
	for idx, data = range mc.mockData {
		if data.served {
			continue
		}
		if len(data.URL) > 0 {
			m, err := regexp.MatchString(data.URL, req.URL.String())
			if err != nil {
				panic("regex pattern issue")
			}
			if !m {
				continue
			}
		}
		// need to change the array element, not the copy in "data"
		mc.mockData[idx].served = true
		mc.lastServed = idx
		found = true
		break
	}

	// default to 200/OK
	statusCode := data.Code
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	if found {
		// log.Printf("%s <%v>, %d: %v\n", req.Method, req.URL, statusCode, string(data.Data))
		log.Printf("%s <%v>, %d: %s\n", req.Method, req.URL, statusCode, data)
	} else {
		for k, v := range mc.mockData {
			log.Printf("%d: %v %v %v\n%v\n%v\n", k, v.served, v.URL, v.Code, sanitizeURL(req.URL.String()), string(v.Data))
			log.Println("**********")
		}
		panic("ran out of data")
	}

	if data.Err != nil {
		return nil, data.Err
	}

	resp := &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader([]byte(data.Data))),
		Header:     make(http.Header),
	}
	return resp, nil
}

// Do satisfies the http.Client.Do() interface
func (m *MockResponder) Do(req *http.Request) (*http.Response, error) {
	// one request at a time!
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.doFunc(req)
}

// SetDoFunc sets a new Do func which again must satisfy the http.Client.Do()
// interface.  If not set, the defaultDoFunc() / built-in doFunc is used.
func (m *MockResponder) SetDoFunc(df func(req *http.Request) (*http.Response, error)) {
	m.doFunc = df
}

// Reset resets the data of the responder so that it can be reused within the
// same test.
func (m *MockResponder) Reset() {
	m.mu.Lock()
	for idx := range m.mockData {
		m.mockData[idx].served = false
	}
	m.lastServed = 0
	m.mu.Unlock()
}

// SetData sets a new mocked data response list into the mock responder.
func (m *MockResponder) SetData(data MockRespList) {
	m.mockData = data
	m.Reset()
}

// GetData returns the currently set mocked data response list.
func (m *MockResponder) GetData() MockRespList {
	return m.mockData
}

// LastData retrieves the mocked data response which was last served.
func (m *MockResponder) LastData() []byte {
	return m.mockData[m.lastServed].Data
}

// Empty returns true if all data in the mocked response list has been served.
// This can be useful at the end of the test to ensure that all data has been
// consumed which typically should be the case after a test run.
func (m *MockResponder) Empty() bool {
	for _, d := range m.mockData {
		if !d.served {
			log.Println(d)
			return false
		}
	}
	return true
}

// NewMockResponder returns a new mock responder and the accompanying context.
// During a request, the mock responder can be retrieved via the context key.
func NewMockResponder() (*MockResponder, context.Context) {
	mc := &MockResponder{
		doFunc:   defaultDoFunc,
		mockData: nil,
	}
	return mc, context.WithValue(context.TODO(), contextMockClient, mc)
}
