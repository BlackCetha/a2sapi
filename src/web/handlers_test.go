package web

// Tests for handler functions for API

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/syncore/a2sapi/src/config"
	"github.com/syncore/a2sapi/src/constants"
	"github.com/syncore/a2sapi/src/db"
	"github.com/syncore/a2sapi/src/models"
	"github.com/syncore/a2sapi/src/test"
	"github.com/syncore/a2sapi/src/util"

	"github.com/gorilla/mux"
)

var testURLBase string

// ResponseRecorder is an extension of httptest.ResponseRecoder which is an
// implementation of http.ResponseWriter that records its mutations for later
// inspection in tests. Credit to ome on freenode #go-nuts.
type ResponseRecoder struct {
	*httptest.ResponseRecorder
}

func init() {
	test.SetupEnvironment()
	testURLBase = fmt.Sprintf("http://:%d", config.Config.WebConfig.APIWebPort)
	db.InitDBs()

	// create dump server file
	err := util.CreateDirectory(constants.DumpDirectory)
	if err != nil {
		panic("Unable to create dump directory used in tests")
	}
	err = util.CreateByteFile(constants.TestServerDumpJSON, constants.DumpFileFullPath(
		config.Config.DebugConfig.ServerDumpFilename), true)
	if err != nil {
		panic(fmt.Sprintf("Test dump file creation error: %s", err))
	}

	// launch server
	go func() {
		r := mux.NewRouter().StrictSlash(true)
		for _, ar := range apiRoutes {
			var handler http.Handler
			handler = compressGzip(ar.handlerFunc, config.Config.WebConfig.CompressResponses)

			r.Methods(ar.method).
				MatcherFunc(pathQStrToLowerMatcherFunc(r, ar.path, ar.queryStrings,
					getRequiredQryStringCount(ar.queryStrings))).
				Name(ar.name).
				Handler(http.TimeoutHandler(handler,
					time.Duration(config.Config.WebConfig.APIWebTimeout)*time.Second,
					`{"error":"Timeout"}`))
		}
		err := http.ListenAndServe(fmt.Sprintf(":%d", config.Config.WebConfig.APIWebPort), r)
		if err != nil {
			panic("Unable to start web server")
		}
	}()
}

func formatURL(path string) string {
	return fmt.Sprintf("%s/%s", testURLBase, path)
}

// newRecorder returns an initialized ResponseRecorder, it's compatiable with
//the official httptest.ResponseRecoder by embedding it.
func newRecorder() *ResponseRecoder {
	return &ResponseRecoder{httptest.NewRecorder()}
}

// ExpectJSON checks if decoding the body to `model` will match the `expect`
// object.
func (r *ResponseRecoder) ExpectJSON(model, expect interface{}) ([]byte, bool) {
	mt := reflect.TypeOf(model)
	me := reflect.TypeOf(expect)
	if me != mt {
		return nil, false
	}
	err := json.Unmarshal(r.Body.Bytes(), model)
	return r.Body.Bytes(), err == nil && reflect.DeepEqual(model, expect)
}

// TestGetServers tests the GetServers HTTP handler
func TestGetServers(t *testing.T) {
	r, _ := http.NewRequest("GET", formatURL("servers"), nil)
	w := newRecorder()
	getServers(w, r)
	// body json test
	m := &models.APIServerList{}
	_, modelMatches := w.ExpectJSON(m, m)
	if !modelMatches {
		t.Errorf("getServers: expected and actual models do not match.")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code: %v for GetServers handler; got: %v",
			http.StatusOK, w.Code)
	}
	if len(w.Body.Bytes()) == 0 {
		t.Errorf("Response body should not be empty")
	}
}

// TestGetServerID tests the GetServerID HTTP handler
func TestGetServerIDs(t *testing.T) {
	r, _ := http.NewRequest("GET", formatURL("serverIDs?hosts=127.0.0.1:65534"),
		nil)
	w := newRecorder()
	getServerIDs(w, r)
	// body json test
	m := &models.DbServerID{}
	_, modelMatches := w.ExpectJSON(m, m)
	if !modelMatches {
		t.Errorf("getServerID: expected and actual models do not match.")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code: %v for GetServerID handler; got: %v",
			http.StatusOK, w.Code)
	}
	if len(w.Body.Bytes()) == 0 {
		t.Errorf("GetServerID handler body should not be empty")
	}
}

// TestQueryServerID tests the QueryServerID HTTP handler
func TestQueryServerID(t *testing.T) {
	r, _ := http.NewRequest("GET", formatURL("query?ids=788593993848"),
		nil)
	w := newRecorder()
	queryServerIDs(w, r)
	// body json test
	m := &models.APIServerList{}
	_, modelMatches := w.ExpectJSON(m, m)
	if !modelMatches {
		t.Errorf("queryServerID: expected and actual models do not match.")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %v for queryServerID handler; got: %v",
			http.StatusOK, w.Code)
	}
	if len(w.Body.Bytes()) == 0 {
		t.Errorf("queryServerID handler body should not be empty")
	}
}

// TestQueryServerAddr tests the QueryServerAddr handler
func TestQueryServerAddr(t *testing.T) {
	r1, _ := http.NewRequest("GET",
		formatURL("query?hosts=127.0.0.1:65534"), nil)
	w1 := newRecorder()
	queryServerAddrs(w1, r1)
	// 200 - default server list
	if w1.Code != http.StatusOK {
		t.Errorf("Expected status code %v for queryServerAddr handler; got: %v",
			http.StatusOK, w1.Code)
	}
	if len(w1.Body.Bytes()) == 0 {
		t.Errorf("queryServerAddr handler body should not be empty")
	}
	// body 1 json test
	m1 := &models.APIServerList{}
	_, modelMatches := w1.ExpectJSON(m1, m1)
	if !modelMatches {
		t.Errorf("queryServerAddr: expected and actual models do not match.")
	}
	// no address specified
	r2, _ := http.NewRequest("GET", formatURL("query?hosts="), nil)
	w2 := newRecorder()
	queryServerAddrs(w2, r2)
	// body 2 json test
	m2 := &models.APIServerList{}
	_, modelMatches2 := w2.ExpectJSON(m2, m2)
	if !modelMatches2 {
		t.Errorf("queryServerAddr: expected and actual models do not match.")
	}
	if w2.Code != http.StatusOK {
		t.Errorf("Expected status code %v for queryServerAddr handler; got: %v",
			http.StatusOK, w2.Code)
	}
	if len(w2.Body.Bytes()) == 0 {
		t.Errorf("queryServerAddr handler body should not be empty")
	}
}
