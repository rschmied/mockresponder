[![Go Reference](https://pkg.go.dev/badge/github.com/rschmied/mockresponder.svg)](https://pkg.go.dev/github.com/rschmied/mockresponder)[![CodeQL](https://github.com/rschmied/mockresponder/actions/workflows/codeql.yml/badge.svg)](https://github.com/rschmied/mockresponder/actions/workflows/codeql.yml) [![Go](https://github.com/rschmied/mockresponder/actions/workflows/go.yml/badge.svg)](https://github.com/rschmied/mockresponder/actions/workflows/go.yml) [![Coverage Status](https://coveralls.io/repos/github/rschmied/mockresponder/badge.svg?branch=main)](https://coveralls.io/github/rschmied/mockresponder?branch=main)[![Go Report Card](https://goreportcard.com/badge/github.com/rschmied/mockresponder)](https://goreportcard.com/report/github.com/rschmied/mockresponder)

# mockresponder

A simple HTTP responder

This package allows to provide mock (API) responses to client code.  The responder
has a canned set of responses which can be matched against URL matched via regex.
Each response can include a status code (default is 200), returned data as `[]byte`,
and a potential `error`.  Each response is only served once, if the provided data
runs out of responses, it will panic.

For this to work, our API client needs to use an interface which satisfies the `http.Client` as well
as our mock responder.

Something like this goes into the API client code:

```go
type apiClient interface {
    Do(req *http.Request) (*http.Response, error)
}

type Client struct {
    host             string
    apiToken         string
    httpClient       apiClient
}
```

Normal operation would then store a `http.Client` instance into the
`httpClient`.  When debugging, though, this can be initialized with an instance
of `mockResponder` and this will then allow to provide mocked responses.

Here's an example of a unit test:

```go
func TestClient_token_auth(t *testing.T) {

    // returns a Client as defined above, having an apiClient attribute
    c := NewClient("https://bla.bla")
    mrClient, ctx := mr.NewMockResponder()
    c.httpClient = mrClient

    tests := []struct {
        name      string
        responses mr.MockRespList
        wantErr   bool
        errstr    string
    }{
        {
            "goodtoken",
            mr.MockRespList{
                mr.MockResp{
                    Data: []byte(`"OK"`),
                },
                mr.MockResp{
                    Data: []byte(`{"version": "2.4.1","ready": true}`),
                },
            },
            false,
            "",
        },
        {
            "badjson",
            mr.MockRespList{
                mr.MockResp{
                    Data: []byte(`,,,`),
                },
            },
            true,
            "invalid character ',' looking for beginning of value",
        },
        {
            "badtoken",
            mr.MockRespList{
                mr.MockResp{
                    Data: []byte(`{
                        "description": "No authorization token provided.",
                        "code": 401
                    }`),
                    Code: 401,
                },
            },
            true,
            "invalid token but no credentials provided",
        },
        {
            "clienterror",
            mr.MockRespList{
                mr.MockResp{
                    Data: []byte{},
                    Err:  errors.New("ka-boom"),
                },
            },
            true,
            "ka-boom",
        },
    }
    for _, tt := range tests {
        mrClient.SetData(tt.responses)
        var err error
        t.Run(tt.name, func(t *testing.T) {
            if err = c.versionCheck(ctx); (err != nil) != tt.wantErr {
                t.Errorf("Client.versionCheck() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
        if !mrClient.Empty() {
            t.Error("not all data in mock client consumed")
        }
        if tt.wantErr {
            assert.EqualError(t, err, tt.errstr)
        }
    }
}
```

(c) 2022 Ralph Schmieder
