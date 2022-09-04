package toolkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

func TestTools_RandomString(t *testing.T) {
	var testTools Tools

	s := testTools.RandomString(10)

	if len(s) != 10 {
		t.Error("Wrong lenght random string returned")
	}
}

var uploadTests = []struct {
	name          string
	allowedTypes  []string
	renameFile    bool
	errorExpected bool
}{
	{name: "allowed no rename", allowedTypes: []string{"image/jpeg", "image/png"}, renameFile: false, errorExpected: false},
	{name: "allowed rename", allowedTypes: []string{"image/jpeg", "image/png"}, renameFile: true, errorExpected: false},
	{name: "not allowed", allowedTypes: []string{"image/jpeg"}, renameFile: false, errorExpected: true},
}

func TestTools_UploadFiles(t *testing.T) {
	for _, entry := range uploadTests {
		// set up a pipe to avoid buffering

		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)
		wg := sync.WaitGroup{}
		wg.Add(1)

		go func() {
			defer writer.Close()
			defer wg.Done()

			// create the form data field 'file'
			part, err := writer.CreateFormFile("file", "./testdata/img.png")

			if err != nil {
				t.Error(err)
			}

			f, err := os.Open("./testdata/img.png")
			if err != nil {
				t.Error(err)
			}
			defer f.Close()

			img, _, err := image.Decode(f)
			if err != nil {
				t.Error("error decoding image", err)
			}

			err = png.Encode(part, img)
			if err != nil {
				t.Error(err)
			}
		}()

		// read from the pipe wich receives data
		request := httptest.NewRequest("POST", "/", pr)
		request.Header.Add("Content-Type", writer.FormDataContentType())

		var testTools Tools

		testTools.AllowedFileTypes = entry.allowedTypes

		uploadedFiles, err := testTools.UploadFiles(request, "./testdata/uploads/", entry.renameFile)

		if err != nil && !entry.errorExpected {
			t.Error(err)
		}

		if !entry.errorExpected {
			if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].NewFileName)); os.IsNotExist(err) {
				t.Errorf("%s: expected file to exists: %s", entry.name, err.Error())
			}

			// clean up
			os.Remove(fmt.Sprintf("./testdata/uploads/%s", uploadedFiles[0].NewFileName))
		}

		if entry.errorExpected && err == nil {
			t.Errorf("%s: error expected but none received", entry.name)
		}

		wg.Wait()
	}
}

func TestTools_UploadOneFile(t *testing.T) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer writer.Close()

		// create the form data field 'file'
		part, err := writer.CreateFormFile("file", "./testdata/img.png")

		if err != nil {
			t.Error(err)
		}

		f, err := os.Open("./testdata/img.png")
		if err != nil {
			t.Error(err)
		}
		defer f.Close()

		img, _, err := image.Decode(f)
		if err != nil {
			t.Error("error decoding image", err)
		}

		err = png.Encode(part, img)
		if err != nil {
			t.Error(err)
		}
	}()

	// read from the pipe wich receives data
	request := httptest.NewRequest("POST", "/", pr)
	request.Header.Add("Content-Type", writer.FormDataContentType())

	var testTools Tools

	uploadedFile, err := testTools.UploadOneFile(request, "./testdata/uploads/")

	if err != nil {
		t.Error(err)
	}

	if _, err := os.Stat(fmt.Sprintf("./testdata/uploads/%s", uploadedFile.NewFileName)); os.IsNotExist(err) {
		t.Errorf("expected file to exists: %s", err.Error())
	}

	// clean up
	os.Remove(fmt.Sprintf("./testdata/uploads/%s", uploadedFile.NewFileName))

}

func TestTools_CreateDirIfNotExists(t *testing.T) {
	var testTool Tools

	err := testTool.CreateDirIfNotExists("./testdata/myDir")
	if err != nil {
		t.Error(err)
	}

	err = testTool.CreateDirIfNotExists("./testdata/myDir")
	if err != nil {
		t.Error(err)
	}

	os.Remove("./testdata/myDir")
}

var slugTests = []struct {
	name          string
	toSlugify     string
	expected      string
	errorExpected bool
}{
	{name: "valid string", toSlugify: "now is the time", expected: "now-is-the-time", errorExpected: false},
	{name: "empty string", toSlugify: "", expected: "", errorExpected: true},
	{name: "complex string", toSlugify: "Now is the time for all GOOD men! + fish & such &^123", expected: "now-is-the-time-for-all-good-men-fish-such-123", errorExpected: false},
	{name: "japanese string", toSlugify: "こんにちは世界", expected: "", errorExpected: true},
	{name: "japanese string", toSlugify: "hello worldこんにちは世界", expected: "hello-world", errorExpected: false},
}

func TestTools_Slugify(t *testing.T) {
	var testTool Tools

	for _, entry := range slugTests {
		slugified, err := testTool.Slugify(entry.toSlugify)
		if err != nil && !entry.errorExpected {
			t.Errorf("%s: error received when none expected: %s", entry.name, err.Error())
		}

		if slugified != entry.expected && !entry.errorExpected {
			t.Errorf("expected %s, but got %s", entry.expected, slugified)
		}
	}
}

func TestTools_DownloadStaticFile(t *testing.T) {
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)

	var testTool Tools

	testTool.DownloadStaticFile(rr, req, "./testdata", "pic.jpg", "puppy.jpg")

	res := rr.Result()
	defer res.Body.Close()

	if length := res.Header["Content-Length"][0]; length != "98827" {
		t.Error("wrong content length of", length)
	}

	if disposition := res.Header["Content-Disposition"][0]; disposition != "attachment; filename=\"puppy.jpg\"" {
		t.Error("wrong content disposition:", disposition)
	}

	_, err := io.ReadAll(res.Body)

	if err != nil {
		t.Error(err)
	}
}

var jsonTests = []struct {
	name          string
	json          string
	errorExpected bool
	maxSize       int
	allowUnknown  bool
}{
	{name: "good json", json: `{"foo":"bar"}`, errorExpected: false, maxSize: 1024, allowUnknown: false},
	{name: "badly formated json", json: `{"foo":}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "incorrect type", json: `{"foo":1}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "two json files", json: `{"foo":"bar"},{"alpha":"2"}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "empty body", json: ``, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "syntax error in json", json: `{"foo":1"}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "unknown field in json", json: `{"fooo":"bar"}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "allowed unknown field in json", json: `{"fooo":"bar"}`, errorExpected: false, maxSize: 1024, allowUnknown: true},
	{name: "missing field name in json", json: `{"jack":"bar"}`, errorExpected: false, maxSize: 1024, allowUnknown: true},
	{name: "file too large", json: `{"foo":"bar"}`, errorExpected: true, maxSize: 1, allowUnknown: false},
	{name: "not json", json: `Hello world`, errorExpected: true, maxSize: 1024, allowUnknown: false},
}

func TestTools_ReadJson(t *testing.T) {
	var testTool Tools
	for _, entry := range jsonTests {
		// set the max file size
		testTool.MaxJSONSize = entry.maxSize

		// Allow or Desallow Unknown Fields
		testTool.AllowUnknownFields = entry.allowUnknown

		// declare a variable to read the decoded json into
		var decodedJSON struct {
			Foo string `json:"foo"`
		}

		// create a request with the body
		req, err := http.NewRequest("POST", "/", bytes.NewReader([]byte(entry.json)))

		if err != nil {
			t.Log("Error:", err)
		}

		defer req.Body.Close()

		rr := httptest.NewRecorder()

		err = testTool.ReadJson(rr, req, &decodedJSON)

		if entry.errorExpected && err == nil {
			t.Errorf("%s: error expected, but none received", entry.name)
		}

		if !entry.errorExpected && err != nil {
			t.Errorf("%s: error not expected, but one received: %s", entry.name, err.Error())
		}

	}
}

func TestTools_WriteJSON(t *testing.T) {
	testTools := Tools{}

	rr := httptest.NewRecorder()
	payload := JSONResponse{
		Error:   false,
		Message: "foo",
	}

	headers := make(http.Header)
	headers.Add("FOO", "BAR")

	err := testTools.WriteJSON(rr, http.StatusOK, payload, headers)

	if err != nil {
		t.Errorf("failed to write JSON: %v", err)
	}
}

func TestTools_ErrorJSON(t *testing.T) {
	testTools := Tools{}

	rr := httptest.NewRecorder()
	err := testTools.ErrorJSON(rr, errors.New("some error"), http.StatusServiceUnavailable)

	if err != nil {
		t.Error(err)
	}

	var payload JSONResponse
	decoder := json.NewDecoder(rr.Body)

	err = decoder.Decode(&payload)

	if err != nil {
		t.Error("Received error when decoding JSON", err)
	}

	if !payload.Error {
		t.Error("error set to false in JSON, but should be true")
	}

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("wrong status code returned; expected %d, but got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func TestTools_PushJSONToRemote(t *testing.T) {
	client := NewTestClient(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("ok")),
			Header:     make(http.Header),
		}
	})

	var testTools Tools
	var foo struct {
		Bar string `json:"bar"`
	}

	foo.Bar = "bar"

	_, _, err := testTools.PushJSONToRemote("http://example.com/some/path", foo, client)

	if err != nil {
		t.Error("failed to call remote url:", err)
	}
}
