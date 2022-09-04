package toolkit

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	randomStringSource = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVXYZ0123456789_+"
)

// Tools is the type used to instantiate this module. Any variable of this type will have access
// to all the methods with the reciever *Tools
type Tools struct {
	MaxFileSize        int
	AllowedFileTypes   []string
	MaxJSONSize        int
	AllowUnknownFields bool
}

/**
 * Cria 2 variáveis s, que é um slice de runes com o tamanho passado e r que será um slice de runes baseado na string passada
 * Itera sobre o s, pegando apenas o index
 * p recebe um inteiro randomico
 * x recebe p como um uint64 e y recebe o tamanho de r como um uint64
 * s recebe no seu index a rune que está no slice r na posição resultante do calculo de módulos
 * retorna o slice de runes s como uma string
 */
// RandomString returns a string of random characters of lenght n, using randomStringSource
// as the source for the string
func (t *Tools) RandomString(size int) string {
	s, r := make([]rune, size), []rune(randomStringSource)

	for i := range s {
		p, _ := rand.Prime(rand.Reader, len(r))
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}

	return string(s)
}

// UploadedFile is a struct used to save information about an uploaded file
type UploadedFile struct {
	NewFileName      string
	OriginalFileName string
	FileSize         int64
}

type UploadFilesParams struct {
	uploadedFiles []*UploadedFile
	header        *multipart.FileHeader
	t             *Tools
	renameFile    bool
	uploadDir     string
}

func uploadFiles(ufp UploadFilesParams) ([]*UploadedFile, error) {
	uploadedFiles, header, t, renameFile, uploadDir := ufp.uploadedFiles, ufp.header, ufp.t, ufp.renameFile, ufp.uploadDir

	var uploadedFile UploadedFile
	inFile, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer inFile.Close()

	buff := make([]byte, 512)
	_, err = inFile.Read(buff)
	if err != nil {
		return nil, err
	}

	//check to see if the file type is permitted
	allowed := false
	fileType := http.DetectContentType(buff)

	if len(t.AllowedFileTypes) > 0 {
		for _, typeReceived := range t.AllowedFileTypes {
			if strings.EqualFold(fileType, typeReceived) {
				allowed = true
			}
		}
	} else {
		allowed = true
	}

	if !allowed {
		return nil, errors.New("the uploaded file type is not permitted")
	}

	_, err = inFile.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	uploadedFile.OriginalFileName = header.Filename

	if renameFile {
		uploadedFile.NewFileName = fmt.Sprintf("%s%s", t.RandomString(25), filepath.Ext(header.Filename))
	} else {
		uploadedFile.NewFileName = header.Filename
	}

	var outfile *os.File
	defer outfile.Close()

	if outfile, err = os.Create(filepath.Join(uploadDir, uploadedFile.NewFileName)); err != nil {
		return nil, err
	} else {
		fileSize, err := io.Copy(outfile, inFile)
		if err != nil {
			return nil, err
		}
		uploadedFile.FileSize = fileSize
	}

	uploadedFiles = append(uploadedFiles, &uploadedFile)
	return uploadedFiles, nil
}

func (t *Tools) UploadFiles(r *http.Request, uploadDir string, rename ...bool) ([]*UploadedFile, error) {
	renameFile := shouldRenameFile(rename...)

	var uploadedFiles []*UploadedFile

	if t.MaxFileSize == 0 {
		t.MaxFileSize = int(math.Pow(1024, 3))
	}

	err := t.CreateDirIfNotExists(uploadDir)

	if err != nil {
		return nil, err
	}

	err = r.ParseMultipartForm(int64(t.MaxFileSize))

	if err != nil {
		return nil, errors.New("the uploaded file is too big")
	}

	for _, fHeaders := range r.MultipartForm.File {
		for _, header := range fHeaders {
			uploadedFiles, err = uploadFiles(UploadFilesParams{uploadedFiles, header, t, renameFile, uploadDir})
			if err != nil {
				return uploadedFiles, err
			}
		}
	}
	return uploadedFiles, nil
}

func (t *Tools) UploadOneFile(r *http.Request, uploadDir string, rename ...bool) (*UploadedFile, error) {
	renameFile := shouldRenameFile(rename...)

	files, err := t.UploadFiles(r, uploadDir, renameFile)

	if err != nil {
		return nil, err
	}

	return files[0], nil
}

// CreateDirIfNotExists create a directory and all necessary parents if it does not exists
func (t *Tools) CreateDirIfNotExists(path string) error {
	const mode = 0755

	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, mode)
		if err != nil {
			return err
		}
	}
	return nil
}

// Slugify is a mean of creating a slug from a string
func (t *Tools) Slugify(s string) (string, error) {
	if s == "" {
		return "", errors.New("empty string not permitted")
	}

	var re = regexp.MustCompile(`[^a-z\d]+`)

	slug := strings.Trim(re.ReplaceAllString(strings.ToLower(s), "-"), "-")

	if len(slug) == 0 {
		return "", errors.New("after removing characters, slug is zero length")
	}
	return slug, nil
}

// DownloadsStatic File downloads a file and tries to force the browser to avoid displaying it in the browser window by setting content disposition.
// It also alllows specification of the display name
func (t *Tools) DownloadStaticFile(w http.ResponseWriter, r *http.Request, pathName, displayName string) {

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", displayName))

	http.ServeFile(w, r, pathName)
}

// JSONResponse is the type used for sending JSON around
type JSONResponse struct {
	Error   bool        `json:"error"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ReadJSON tries to read the body of a request and converts from json into a go data variable
func (t *Tools) ReadJson(w http.ResponseWriter, r *http.Request, data interface{}) error {
	maxBytes := 1024 * 1024 // one mega

	if t.MaxJSONSize != 0 {
		maxBytes = t.MaxJSONSize
	}

	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))

	dec := json.NewDecoder(r.Body)

	if !t.AllowUnknownFields {
		dec.DisallowUnknownFields()
	}

	err := dec.Decode(data)

	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed JSON (at character %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)
		case errors.Is(err, io.EOF):
			return errors.New("body must not be empty")
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field")
			return fmt.Errorf("body contains unknown key %s", fieldName)
		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)
		case errors.As(err, &invalidUnmarshalError):
			return fmt.Errorf("error unmarshalling JSON: %s", err.Error())
		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})

	if err != io.EOF {
		return errors.New("body must contain only one JSON value")
	}

	return nil
}

// WriteJSON takes a response status code and arbitrary data and writes json to the client
func (t *Tools) WriteJSON(w http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
	out, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(out)
	if err != nil {
		return err
	}
	return nil
}

// ErrorJSON takes an error and optionally a status code, and generates and sends a JSON error message
func (t *Tools) ErrorJSON(w http.ResponseWriter, err error, status ...int) error {
	statusCode := http.StatusBadRequest

	if len(status) > 0 {
		statusCode = status[0]
	}

	var payload JSONResponse

	payload.Error = true
	payload.Message = err.Error()

	return t.WriteJSON(w, statusCode, payload)
}

// PushJSONToRemote post arbitrary data to some url as JSON, and returns the response, status code, and error, if any.
// The final parameter, client is optional. If none is specified we use the standard http.Client
func (t *Tools) PushJSONToRemote(uri string, data interface{}, client ...*http.Client) (*http.Response, int, error) {
	// create json
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, 0, err
	}
	// check for custom http client
	httpClient := &http.Client{}
	if len(client) > 0 {
		httpClient = client[0]
	}
	// build the request and set the header
	request, err := http.NewRequest("POST", uri, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, 0, err
	}
	request.Header.Set("Content-Type", "application/json")
	// call the remote uri
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	//send the response back
	return response, response.StatusCode, nil
}

func shouldRenameFile(rename ...bool) bool {
	if len(rename) > 0 {
		return rename[0]
	}
	return true
}
